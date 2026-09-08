//go:build linux

package menubar

/*
#cgo linux pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>
#include <gdk/gdkkeysyms.h>

extern void goOnKioskExit();

static GtkWidget *cached_webview = NULL;

static gboolean on_context_menu_cb(GtkWidget *widget, gpointer context_menu, gpointer event, gpointer hit_test_result, gpointer user_data) {
    return TRUE; // Returning TRUE cancels the context menu completely
}

static void on_script_message_cb(WebKitUserContentManager *manager, WebKitJavascriptResult *result, gpointer user_data) {
    goOnKioskExit();
}

static gboolean on_decide_policy_cb(WebKitWebView *web_view, WebKitPolicyDecision *decision, WebKitPolicyDecisionType type, gpointer user_data) {
    if (type == WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION) {
        WebKitNavigationPolicyDecision *nav_decision = WEBKIT_NAVIGATION_POLICY_DECISION(decision);
        WebKitNavigationAction *action = webkit_navigation_policy_decision_get_navigation_action(nav_decision);
        WebKitURIRequest *req = webkit_navigation_action_get_request(action);
        if (req) {
            const gchar *uri = webkit_uri_request_get_uri(req);
            if (uri && (g_strrstr(uri, "kiosk_exit=1") || g_strrstr(uri, "/api/kiosk/exit"))) {
                webkit_policy_decision_ignore(decision);
                goOnKioskExit();
                return TRUE;
            }
        }
    }
    return FALSE;
}

static gboolean on_key_press_cb(GtkWidget *widget, GdkEventKey *event, gpointer user_data) {
    if (!event) return FALSE;
    if (event->keyval == GDK_KEY_Escape || event->keyval == GDK_KEY_F11) {
        goOnKioskExit();
        return TRUE;
    }
    return FALSE;
}

static void setup_webview(GtkWidget *widget) {
    if (!widget) return;
    GType type = G_OBJECT_TYPE(widget);
    const gchar *type_name = g_type_name(type);
    if (type_name && g_str_has_prefix(type_name, "WebKitWebView")) {
        cached_webview = widget;

        g_signal_handlers_disconnect_by_func(widget, G_CALLBACK(on_context_menu_cb), NULL);
        g_signal_connect(widget, "context-menu", G_CALLBACK(on_context_menu_cb), NULL);

        g_signal_handlers_disconnect_by_func(widget, G_CALLBACK(on_key_press_cb), NULL);
        g_signal_connect(widget, "key-press-event", G_CALLBACK(on_key_press_cb), NULL);

        g_signal_handlers_disconnect_by_func(widget, G_CALLBACK(on_decide_policy_cb), NULL);
        g_signal_connect(widget, "decide-policy", G_CALLBACK(on_decide_policy_cb), NULL);

        WebKitUserContentManager *manager = webkit_web_view_get_user_content_manager(WEBKIT_WEB_VIEW(widget));
        if (manager) {
            webkit_user_content_manager_unregister_script_message_handler(manager, "eposProxyExit");
            webkit_user_content_manager_register_script_message_handler(manager, "eposProxyExit");
            g_signal_handlers_disconnect_by_func(manager, G_CALLBACK(on_script_message_cb), NULL);
            g_signal_connect(manager, "script-message-received::eposProxyExit", G_CALLBACK(on_script_message_cb), NULL);
        }
        return;
    }
    if (GTK_IS_CONTAINER(widget)) {
        gtk_container_forall(GTK_CONTAINER(widget), (GtkCallback)setup_webview, NULL);
    }
}

static gboolean setup_all_windows_idle(gpointer data) {
    GList *toplevels = gtk_window_list_toplevels();
    for (GList *l = toplevels; l != NULL; l = l->next) {
        if (GTK_IS_WINDOW(l->data)) {
            g_signal_handlers_disconnect_by_func(l->data, G_CALLBACK(on_key_press_cb), NULL);
            g_signal_connect(l->data, "key-press-event", G_CALLBACK(on_key_press_cb), NULL);
            setup_webview(GTK_WIDGET(l->data));
        }
    }
    g_list_free(toplevels);
    return G_SOURCE_REMOVE;
}

static void setup_all_windows() {
    g_idle_add(setup_all_windows_idle, NULL);
}

static void find_and_set_menubar_visibility(GtkWidget *widget, gpointer data) {
    gboolean visible = GPOINTER_TO_INT(data);
    if (GTK_IS_MENU_BAR(widget)) {
        if (visible) {
            gtk_widget_show(widget);
        } else {
            gtk_widget_hide(widget);
        }
        return;
    }
    if (GTK_IS_CONTAINER(widget)) {
        gtk_container_forall(GTK_CONTAINER(widget), find_and_set_menubar_visibility, data);
    }
}

static gboolean set_all_menubars_visible_idle(gpointer data) {
    GList *toplevels = gtk_window_list_toplevels();
    for (GList *l = toplevels; l != NULL; l = l->next) {
        if (GTK_IS_WINDOW(l->data)) {
            find_and_set_menubar_visibility(GTK_WIDGET(l->data), data);
        }
    }
    g_list_free(toplevels);
    return G_SOURCE_REMOVE;
}

static void set_menubars_visible(int visible) {
    g_idle_add(set_all_menubars_visible_idle, GINT_TO_POINTER(visible));
}

static gboolean navigate_webview_idle(gpointer data) {
    gchar *url = (gchar*)data;
    if (cached_webview && WEBKIT_IS_WEB_VIEW(cached_webview)) {
        webkit_web_view_load_uri(WEBKIT_WEB_VIEW(cached_webview), url);
    }
    g_free(url);
    return G_SOURCE_REMOVE;
}

static void navigate_to_url(const char *url) {
    if (!url) return;
    g_idle_add(navigate_webview_idle, g_strdup(url));
}
*/
import "C"
import "unsafe"

//export goOnKioskExit
func goOnKioskExit() {
	if nativeKioskExitCb != nil {
		go nativeKioskExitCb()
	}
}

var nativeKioskExitCb func()

// SetNativeKioskExitCallback installs the native WebKit and GTK hooks to detect 4 corner taps and Escape/F11.
func SetNativeKioskExitCallback(cb func()) {
	nativeKioskExitCb = cb
	C.setup_all_windows()
}

// DisableContextMenu disables native WebKitGTK right-click context menus and wires event listeners.
func DisableContextMenu() {
	C.setup_all_windows()
}

// SetNativeMenubarVisible toggles the visibility of the native GTK menubar on Linux.
func SetNativeMenubarVisible(visible bool) {
	if visible {
		C.set_menubars_visible(1)
	} else {
		C.set_menubars_visible(0)
	}
}

// NavigateToURL natively loads the specified URI directly into the WebKitWebView.
func NavigateToURL(targetURL string) {
	cURL := C.CString(targetURL)
	defer C.free(unsafe.Pointer(cURL))
	C.navigate_to_url(cURL)
}
