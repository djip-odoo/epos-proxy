//go:build android && cgo

package printer

/*
#include <jni.h>
#include <stdlib.h>

static JavaVM* g_jvm = NULL;
static jobject g_bridge = NULL;

static void storePrinterBridgeRef(JNIEnv *env, jobject bridge) {
    if ((*env)->GetJavaVM(env, &g_jvm) != 0) {
        return;
    }
    g_bridge = (*env)->NewGlobalRef(env, bridge);
}

static char* callJavaPrinterMethod(const char* methodName, const char* arg) {
    if (g_bridge == NULL || g_jvm == NULL) return NULL;
    JNIEnv* env = NULL;
    int detach = 0;
    jint r = (*g_jvm)->GetEnv(g_jvm, (void**)&env, JNI_VERSION_1_6);
    if (r == JNI_EDETACHED) {
        if ((*g_jvm)->AttachCurrentThread(g_jvm, &env, NULL) != 0) {
            return NULL;
        }
        detach = 1;
    } else if (r != JNI_OK) {
        return NULL;
    }

    char* result = NULL;
    jclass cls = (*env)->GetObjectClass(env, g_bridge);
    if (cls != NULL) {
        jmethodID mid = (*env)->GetMethodID(env, cls, methodName, "(Ljava/lang/String;)Ljava/lang/String;");
        if (mid != NULL) {
            jstring jarg = (*env)->NewStringUTF(env, arg);
            jstring jresult = (jstring)(*env)->CallObjectMethod(env, g_bridge, mid, jarg);
            if (jresult != NULL) {
                const char* chars = (*env)->GetStringUTFChars(env, jresult, NULL);
                if (chars != NULL) {
                    result = strdup(chars);
                    (*env)->ReleaseStringUTFChars(env, jresult, chars);
                }
                (*env)->DeleteLocalRef(env, jresult);
            }
            if (jarg != NULL) (*env)->DeleteLocalRef(env, jarg);
        }
        (*env)->DeleteLocalRef(env, cls);
    }

    if (detach) {
        (*g_jvm)->DetachCurrentThread(g_jvm);
    }
    return result;
}
*/
import "C"

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"
	"epos-proxy/logger"
)

//export Java_com_wails_app_WailsBridge_nativeInitPrinterBridge
func Java_com_wails_app_WailsBridge_nativeInitPrinterBridge(env *C.JNIEnv, obj C.jobject, bridge C.jobject) {
	C.storePrinterBridgeRef(env, bridge)
}

func callJavaPrinter(method string, arg string) string {
	cmethod := C.CString(method)
	carg := C.CString(arg)
	defer C.free(unsafe.Pointer(cmethod))
	defer C.free(unsafe.Pointer(carg))
	cresult := C.callJavaPrinterMethod(cmethod, carg)
	if cresult == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cresult))
	return C.GoString(cresult)
}

func (p *Printer) writeUSB(data []byte) error {
	if p.id == nil {
		return errors.New("cannot print to USB: printer ID is nil")
	}

	dataBase64 := base64.StdEncoding.EncodeToString(data)
	payload := map[string]string{
		"path":   p.id.Path,
		"vidPid": p.id.VidPid,
		"serial": p.id.Serial,
		"data":   dataBase64,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize USB print payload: %w", err)
	}

	resJson := callJavaPrinter("printUSB", string(jsonBytes))
	if resJson == "" {
		return errors.New("USB print call returned empty result")
	}

	var res struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	err = json.Unmarshal([]byte(resJson), &res)
	if err != nil {
		return fmt.Errorf("failed to parse USB print result: %w", err)
	}

	if !res.OK {
		return fmt.Errorf("USB print failed: %s", res.Error)
	}

	logger.Debugf("Successfully wrote print job to Android USB printer")
	return nil
}

func (p *Printer) ensureOpenUSBLocked() error {
	if p.id == nil {
		return errors.New("cannot check USB printer: printer ID is nil")
	}
	// On Android, we don't open a persistent connection in ensureOpen because Android USB
	// permissions and connections are handled on-demand per bulk write.
	return nil
}

func (p *Printer) closeUSBDeviceLocked() {
	// Persistent USB connections are not held on Android, so close is a no-op.
}
