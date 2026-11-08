//go:build windows

package report

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	openuem_nats "github.com/open-uem/nats"
)

// pendingUpdateFromDispatch reads the details of a single update from an
// IUpdate IDispatch. The ceshihao/windowsupdate wrapper is not used because it
// parses every property and panics on unexpected VARIANT types (seen on
// Windows Servers with pending updates).
func pendingUpdateFromDispatch(item *ole.IDispatch) openuem_nats.PendingUpdate {
	u := openuem_nats.PendingUpdate{}

	if v, err := oleutil.GetProperty(item, "Title"); err == nil {
		u.Title = v.ToString()
	}

	if v, err := oleutil.GetProperty(item, "KBArticleIDs"); err == nil {
		kbs := v.ToIDispatch()
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
			}
			kbs.Release()
		}
	}

	if v, err := oleutil.GetProperty(item, "IsDownloaded"); err == nil {
		u.IsDownloaded = v.Val != 0
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
