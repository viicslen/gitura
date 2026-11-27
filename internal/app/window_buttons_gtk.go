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

static void gitura_free(char *s) { g_free(s); }

// Both stay 1 when unknown: the window is not realized yet, or GTK predates the capabilities API (4.20).
static void gitura_toplevel_can(void *window, int *minimize, int *maximize) {
	*minimize = *maximize = 1;
#if GTK_CHECK_VERSION(4, 20, 0)
	GdkSurface *surface = gtk_native_get_surface(GTK_NATIVE(window));
	if (!surface || !GDK_IS_TOPLEVEL(surface)) return;
	GdkToplevelCapabilities caps = gdk_toplevel_get_capabilities(GDK_TOPLEVEL(surface));
	*minimize = (caps & GDK_TOPLEVEL_CAPABILITIES_MINIMIZE) != 0;
	*maximize = (caps & GDK_TOPLEVEL_CAPABILITIES_MAXIMIZE) != 0;
#endif
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// ponytail: read once when the strip mounts; listen for notify::capabilities / notify::gtk-decoration-layout if WMs change them live.
func wmWindowButtons() []string {
	w, ok := mainWindow()
	if !ok {
		return allWindowButtons
	}
	layout := "menu:minimize,maximize,close"
	canMinimize, canMaximize := C.int(1), C.int(1)
	application.InvokeSync(func() {
		if l := C.gitura_decoration_layout(); l != nil {
			layout = C.GoString(l)
			C.gitura_free(l)
		}
		if p := w.NativeWindow(); p != nil {
			C.gitura_toplevel_can(p, &canMinimize, &canMaximize)
		}
	})
	return windowButtons(layout, canMinimize != 0, canMaximize != 0)
}
