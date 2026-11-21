package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static WKWebView *shkFindWebView(NSView *v) {
	if ([v isKindOfClass:[WKWebView class]]) return (WKWebView *)v;
	for (NSView *s in v.subviews) {
		WKWebView *r = shkFindWebView(s);
		if (r) return r;
	}
	return nil;
}

static void shkSetPageZoom(double z) {
	dispatch_async(dispatch_get_main_queue(), ^{
		for (NSWindow *w in [NSApp windows]) {
			WKWebView *wv = shkFindWebView(w.contentView);
			if (wv && [wv respondsToSelector:@selector(setPageZoom:)]) {
				wv.pageZoom = z;
				return;
			}
		}
	});
}
*/
import "C"

// setNativePageZoom memakai WKWebView.pageZoom (seperti zoom Safari): seluruh
// koordinat (rect, clientX, elementFromPoint) tetap konsisten, tidak seperti
// CSS `zoom` yang membuat getBoundingClientRect dan clientX beda ruang.
func setNativePageZoom(factor float64) {
	C.shkSetPageZoom(C.double(factor))
}
