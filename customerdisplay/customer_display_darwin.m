// customer_display_darwin.m
// macOS Customer Display — NSWindow + WKWebView
//
// This file implements the customer display backend for macOS using:
//   • NSScreen     — monitor enumeration
//   • NSWindow     — borderless, always-on-top fullscreen window
//   • WKWebView    — hardware-accelerated web renderer
//
// All functions are called from Go via CGo (display_darwin.go) and must
// therefore be plain C-linkage functions.

#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>
#import <Foundation/Foundation.h>
#include <stdlib.h>

// ── Forward declarations of Go callbacks ─────────────────────────────────
extern void goOnMonitorAdded();
extern void goOnMonitorRemoved();

// ── Singleton state ───────────────────────────────────────────────────────
static NSWindow   *customerWindow  = nil;
static WKWebView  *customerWebView = nil;
static NSString   *currentURL      = nil;

// ── Screen-change observer ────────────────────────────────────────────────
@interface CDScreenObserver : NSObject
@end

@implementation CDScreenObserver
- (void)screensDidChange:(NSNotification *)note {
    NSUInteger newCount = [[NSScreen screens] count];
    static NSUInteger prevCount = 0;
    if (newCount > prevCount) {
        goOnMonitorAdded();
    } else if (newCount < prevCount) {
        goOnMonitorRemoved();
    }
    prevCount = newCount;
}
@end

static CDScreenObserver *screenObserver = nil;

// ── Helpers ───────────────────────────────────────────────────────────────

/// Build the monitor-ID string used by Go to identify a specific NSScreen.
/// Format mirrors the Linux GTK implementation:  "mfr-model-WxH-x-y-idx"
static NSString *screenID(NSScreen *screen, NSUInteger idx) {
    NSDictionary *desc   = [screen deviceDescription];
    NSString     *name   = [screen localizedName];
    if (!name || name.length == 0) name = @"Unknown Display";
    CGRect frame = [screen frame];
    return [NSString stringWithFormat:@"Apple-%@-%dx%d-%d-%d-%lu",
            name,
            (int)frame.size.width,
            (int)frame.size.height,
            (int)frame.origin.x,
            (int)frame.origin.y,
            (unsigned long)idx];
}

/// Find the NSScreen that matches a C monitor-ID string, falling back to the
/// first screen when no match is found.
static NSScreen *findScreen(const char *monitor_id) {
    NSString *target = [NSString stringWithUTF8String:monitor_id];
    NSArray<NSScreen *> *screens = [NSScreen screens];
    for (NSUInteger i = 0; i < screens.count; i++) {
        if ([screenID(screens[i], i) isEqualToString:target]) {
            return screens[i];
        }
    }
    return screens.firstObject;
}

// ── Public C functions (called from Go via CGo) ───────────────────────────

void setup_monitor_signals_c(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (screenObserver) return;
        screenObserver = [[CDScreenObserver alloc] init];
        [[NSNotificationCenter defaultCenter]
            addObserver:screenObserver
               selector:@selector(screensDidChange:)
                   name:NSApplicationDidChangeScreenParametersNotification
                 object:nil];
    });
}

char *get_monitors_json_c(void) {
    __block char *result = NULL;
    dispatch_sync(dispatch_get_main_queue(), ^{
        NSArray<NSScreen *> *screens = [NSScreen screens];
        NSScreen *primary = screens.firstObject; // screen 0 is the primary on macOS
        NSMutableString *json = [NSMutableString stringWithString:@"["];

        for (NSUInteger i = 0; i < screens.count; i++) {
            NSScreen *screen = screens[i];
            CGRect frame = [screen frame];
            NSString *name = [screen localizedName];
            if (!name || name.length == 0) name = @"Unknown Display";

            NSString *sid = screenID(screen, i);
            BOOL isPrimary = (screen == primary);

            if (i > 0) [json appendString:@","];
            [json appendFormat:
                @"{\"id\":\"%@\",\"name\":\"%@\",\"width\":%d,\"height\":%d,"
                 "\"x\":%d,\"y\":%d,\"isPrimary\":%@}",
                sid,
                name,
                (int)frame.size.width,
                (int)frame.size.height,
                (int)frame.origin.x,
                (int)frame.origin.y,
                isPrimary ? @"true" : @"false"];
        }
        [json appendString:@"]"];
        result = strdup([json UTF8String]);
    });
    return result;
}

void open_customer_display_c(const char *monitor_id, const char *url) {
    NSString *nsURL = [NSString stringWithUTF8String:url];
    NSString *nsID  = [NSString stringWithUTF8String:monitor_id];

    dispatch_async(dispatch_get_main_queue(), ^{
        NSScreen *screen = findScreen([nsID UTF8String]);
        if (!screen) return;

        CGRect frame = [screen frame];

        if (customerWindow) {
            // Reposition on the (possibly different) monitor
            [customerWindow setFrame:frame display:YES];

            // Navigate if URL changed
            if (![currentURL isEqualToString:nsURL]) {
                currentURL = [nsURL copy];
                NSURL *u = [NSURL URLWithString:currentURL];
                if (u) [customerWebView loadRequest:[NSURLRequest requestWithURL:u]];
            }
        } else {
            // Create a new borderless, always-on-top fullscreen window
            NSWindow *win = [[NSWindow alloc]
                initWithContentRect:frame
                          styleMask:NSWindowStyleMaskBorderless
                            backing:NSBackingStoreBuffered
                              defer:NO
                             screen:screen];
            [win setLevel:NSScreenSaverWindowLevel];
            [win setCollectionBehavior:
                NSWindowCollectionBehaviorCanJoinAllSpaces |
                NSWindowCollectionBehaviorFullScreenPrimary];
            [win setOpaque:YES];
            [win setHidesOnDeactivate:NO];

            // Embed WKWebView filling the entire window
            WKWebViewConfiguration *cfg = [[WKWebViewConfiguration alloc] init];
            WKWebView *wv = [[WKWebView alloc] initWithFrame:frame configuration:cfg];
            [wv setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
            [win setContentView:wv];

            currentURL = [nsURL copy];
            NSURL *u = [NSURL URLWithString:currentURL];
            if (u) [wv loadRequest:[NSURLRequest requestWithURL:u]];

            [win makeKeyAndOrderFront:nil];
            // Enter native fullscreen on the target screen
            [win toggleFullScreen:nil];

            customerWindow  = win;
            customerWebView = wv;
        }
    });
}

void close_customer_display_c(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (customerWindow) {
            [customerWindow close];
            customerWindow  = nil;
            customerWebView = nil;
            currentURL      = nil;
        }
    });
}

void reload_customer_display_c(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (customerWebView) {
            [customerWebView reload];
        }
    });
}

void navigate_customer_display_c(const char *url) {
    NSString *nsURL = [NSString stringWithUTF8String:url];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (customerWebView && nsURL.length > 0) {
            currentURL = [nsURL copy];
            NSURL *u = [NSURL URLWithString:currentURL];
            if (u) [customerWebView loadRequest:[NSURLRequest requestWithURL:u]];
        }
    });
}

// ── Identify: numbered overlay on every screen for 3 s ───────────────────

void identify_monitors_c(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray<NSScreen *> *screens = [NSScreen screens];
        NSMutableArray<NSWindow *> *wins = [NSMutableArray array];

        for (NSUInteger i = 0; i < screens.count; i++) {
            NSScreen *screen = screens[i];
            CGRect frame = [screen frame];

            NSWindow *win = [[NSWindow alloc]
                initWithContentRect:frame
                          styleMask:NSWindowStyleMaskBorderless
                            backing:NSBackingStoreBuffered
                              defer:NO
                             screen:screen];
            [win setLevel:NSScreenSaverWindowLevel];
            [win setOpaque:YES];

            // Use a WKWebView for the identify overlay
            WKWebView *wv = [[WKWebView alloc] initWithFrame:frame];
            [wv setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
            [win setContentView:wv];

            NSString *html = [NSString stringWithFormat:
                @"<!DOCTYPE html><html><head><style>"
                "body{background:rgba(15,23,42,.95);color:#fff;"
                "font-family:system-ui,-apple-system,sans-serif;"
                "display:flex;justify-content:center;align-items:center;"
                "height:100vh;margin:0;overflow:hidden;}"
                ".n{font-size:240px;font-weight:800;"
                "background:linear-gradient(135deg,#a78bfa,#8b5cf6);"
                "-webkit-background-clip:text;-webkit-text-fill-color:transparent;"
                "filter:drop-shadow(0 10px 20px rgba(139,92,246,.3));}"
                "</style></head><body><div class=\"n\">%lu</div></body></html>",
                (unsigned long)(i + 1)];

            [wv loadHTMLString:html baseURL:nil];
            [win makeKeyAndOrderFront:nil];
            [wins addObject:win];
        }

        // Close all identify windows after 3 s
        dispatch_after(
            dispatch_time(DISPATCH_TIME_NOW, (int64_t)(3 * NSEC_PER_SEC)),
            dispatch_get_main_queue(),
            ^{
                for (NSWindow *w in wins) [w close];
            });
    });
}

// ── Test display: show test screen on a specific monitor for 3 s ──────────

void open_test_display_c(const char *monitor_id) {
    NSString *nsID = [NSString stringWithUTF8String:monitor_id];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSScreen *screen = findScreen([nsID UTF8String]);
        if (!screen) return;

        CGRect frame = [screen frame];

        NSWindow *win = [[NSWindow alloc]
            initWithContentRect:frame
                      styleMask:NSWindowStyleMaskBorderless
                        backing:NSBackingStoreBuffered
                          defer:NO
                         screen:screen];
        [win setLevel:NSScreenSaverWindowLevel];
        [win setOpaque:YES];

        WKWebView *wv = [[WKWebView alloc] initWithFrame:frame];
        [wv setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
        [win setContentView:wv];

        NSString *html =
            @"<!DOCTYPE html><html><head><style>"
            "body{background:#0f172a;color:#fff;"
            "font-family:system-ui,-apple-system,sans-serif;"
            "display:flex;flex-direction:column;"
            "justify-content:center;align-items:center;"
            "height:100vh;margin:0;overflow:hidden;text-align:center;}"
            "h1{font-size:48px;font-weight:800;letter-spacing:.1em;margin-bottom:24px;"
            "background:linear-gradient(135deg,#a78bfa,#8b5cf6);"
            "-webkit-background-clip:text;-webkit-text-fill-color:transparent;}"
            "p{font-size:20px;color:#94a3b8;max-width:600px;line-height:1.6;}"
            ".line{width:80px;height:4px;background:#8b5cf6;margin:32px auto;border-radius:2px;}"
            "</style></head><body>"
            "<h1>CUSTOMER DISPLAY</h1>"
            "<div class=\"line\"></div>"
            "<p>If you can see this screen,<br>the monitor selection is correct.</p>"
            "<p style=\"font-size:14px;color:#64748b;margin-top:40px;\">"
            "This window will close automatically.</p>"
            "</body></html>";

        [wv loadHTMLString:html baseURL:nil];
        [win makeKeyAndOrderFront:nil];

        dispatch_after(
            dispatch_time(DISPATCH_TIME_NOW, (int64_t)(3 * NSEC_PER_SEC)),
            dispatch_get_main_queue(),
            ^{ [win close]; });
    });
}
