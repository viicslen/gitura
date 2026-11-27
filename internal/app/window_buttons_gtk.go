//go:build linux && cgo && !gtk3 && !purego

package app

/*
#cgo pkg-config: gtk4
#include <gtk/gtk.h>

static char *gitura_decoration_layout(void) {
	GtkSettings *settings = gtk_settings_get_default();
	char *layout = NULL;
	if (settings) g_object_get(settings, "gtk-decoration-layout", &layout, NULL);
	return layout;
}

// -1 when unknown: the window is not realized yet, or GTK predates the capabilities API.
static int gitura_toplevel_capabilities(void *window) {
#if GTK_CHECK_VERSION(4, 20, 0)
	GdkSurface *surface = gtk_native_get_surface(GTK_NATIVE(window));
	if (surface && GDK_IS_TOPLEVEL(surface)) return gdk_toplevel_get_capabilities(GDK_TOPLEVEL(surface));
#endif
	return -1;
}
*/
import "C"

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ponytail: read once when the strip mounts; listen for notify::capabilities / notify::gtk-decoration-layout if WMs change them live.
func wmWindowButtons() []string {
	w, ok := mainWindow()
	if !ok {
		return allWindowButtons
	}
	layout := "menu:minimize,maximize,close"
	caps := C.int(-1)
	application.InvokeSync(func() {
		if l := C.gitura_decoration_layout(); l != nil {
			layout = C.GoString(l)
			C.g_free(C.gpointer(unsafe.Pointer(l)))
		}
		if p := w.NativeWindow(); p != nil {
			caps = C.gitura_toplevel_capabilities(p)
		}
	})
	return windowButtons(layout,
		caps < 0 || caps&C.GDK_TOPLEVEL_CAPABILITIES_MINIMIZE != 0,
		caps < 0 || caps&C.GDK_TOPLEVEL_CAPABILITIES_MAXIMIZE != 0)
}
