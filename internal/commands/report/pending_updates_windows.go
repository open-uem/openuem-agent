//go:build windows

package report

import (
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	openuem_nats "github.com/open-uem/nats"
)

// variantToDispatch returns the IDispatch held by the VARIANT, or nil when the
// value is not a dispatch object. ToIDispatch alone cannot be used: it reuses
// the VARIANT value unconditionally, so a VT_UNKNOWN/VT_EMPTY result would hand
// back a pointer that is not an IDispatch and crash on the next COM call.
func variantToDispatch(v *ole.VARIANT) *ole.IDispatch {
	if v == nil || v.VT != ole.VT_DISPATCH {
		return nil
	}
	return v.ToIDispatch()
}

// pendingUpdateFromDispatch reads the details of a single update from an
// IUpdate IDispatch. The ceshihao/windowsupdate wrapper is not used because it
// parses every property and panics on unexpected VARIANT types (seen on
// Windows Servers with pending updates).
//
// Ownership rule: every VARIANT returned by oleutil is released via Clear(),
// never via Release() on the extracted dispatch — releasing both frees the COM
// object twice and corrupts the heap (service crash on report end).
func pendingUpdateFromDispatch(item *ole.IDispatch) (u openuem_nats.PendingUpdate) {
	// A single malformed update must not abort the whole report.
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[ERROR]: recovered from panic parsing pending update: %v\n%s", rec, debug.Stack())
		}
	}()

	if v, err := oleutil.GetProperty(item, "Title"); err == nil {
		u.Title = v.ToString()
		v.Clear()
	}

	if v, err := oleutil.GetProperty(item, "KBArticleIDs"); err == nil {
		kbs := variantToDispatch(v)
		if kbs != nil {
			if c, err := oleutil.GetProperty(kbs, "Count"); err == nil {
				ids := []string{}
				for j := 0; j < int(c.Val); j++ {
					if kv, err := oleutil.GetProperty(kbs, "Item", j); err == nil {
						switch n := kv.Value().(type) {
						case string:
							ids = append(ids, "KB"+n)
						case int32:
							ids = append(ids, "KB"+strconv.FormatInt(int64(n), 10))
						case int64:
							ids = append(ids, "KB"+strconv.FormatInt(n, 10))
						}
						kv.Clear()
					}
				}
				u.KBArticleIDs = strings.Join(ids, ",")
				c.Clear()
			}
		}
		v.Clear()
	}

	if v, err := oleutil.GetProperty(item, "IsDownloaded"); err == nil {
		u.IsDownloaded = v.Val != 0
		v.Clear()
	}

	return u
}

// PendingUpdatesDetailString renders the list as a compact string for logging.
func PendingUpdatesDetailString(list []openuem_nats.PendingUpdate) string {
	parts := make([]string, 0, len(list))
	for _, u := range list {
		state := "not downloaded"
		if u.IsDownloaded {
			state = "downloaded"
		}
		parts = append(parts, fmt.Sprintf("%s (%s, %s)", u.Title, u.KBArticleIDs, state))
	}
	return strings.Join(parts, "; ")
}
