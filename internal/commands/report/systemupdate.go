package report

import (
	"fmt"
	"log"
	"time"

	wu "github.com/ceshihao/windowsupdate"
	"github.com/doncicuto/openuem_nats"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Ref: https://learn.microsoft.com/en-us/windows/win32/api/wuapi/ne-wuapi-automaticupdatesnotificationlevel
type notificationLevel int32

const (
	NOTIFICATION_LEVEL_NOT_CONFIGURED notificationLevel = iota
	NOTIFICATION_LEVEL_DISABLED
	NOTIFICATION_LEVEL_NOTIFY_BEFORE_DOWNLOAD
	NOTIFICATION_LEVEL_NOTIFY_BEFORE_INSTALLATION
	NOTIFICATION_LEVEL_SCHEDULED_INSTALLATION
)

func (r *Report) getSystemUpdateInfo() {
	// Get information about Windows Update settings

	// TODO 1 (security) get information about what SMB client version is installed
	// Maybe we can query Win32_OptionalFeature and check for \\LOTHLORIEN\root\cimv2:Win32_OptionalFeature.Name="SMB1Protocol-Client" and
	// its install state as this is an optional feature

	// TODO 2 (security) check if firewall is enabled in the three possible domains

	if err := r.getWindowsUpdateStatus(); err != nil {
		log.Printf("[ERROR]: could not get windows update status info information from wuapi: %v", err)
	} else {
		log.Printf("[INFO]: windows update status info has been retrieved from wuapi")
	}

	if err := r.getWindowsUpdateDates(); err != nil {
		log.Printf("[ERROR]: could not get windows update dates information from wuapi: %v", err)
	} else {
		log.Printf("[INFO]: windows update dates info has been retrieved from wuapi")
	}

	if err := r.getPendingUpdates(); err != nil {
		log.Printf("[ERROR]: could not get pending updates information from wuapi: %v", err)
	} else {
		log.Printf("[INFO]: pending updates info has been retrieved from wuapi")
	}

	if err := r.getUpdatesHistory(); err != nil {
		log.Printf("[ERROR]: could not get updates history information from wuapi: %v", err)
	} else {
		log.Printf("[INFO]: updates history info has been retrieved from wuapi")
	}
}

func (r *Report) logSystemUpdate() {
	fmt.Printf("\n** 🔄 Updates *******************************************************************************************************\n")
	fmt.Printf("%-40s |  %s \n", "Automatic Updates status", r.SystemUpdate.Status)
	if r.SystemUpdate.LastInstall.IsZero() {
		fmt.Printf("%-40s |  %s \n", "Last updates installation date", "Unknown")
	} else {
		fmt.Printf("%-40s |  %v \n", "Last updates installation date", r.SystemUpdate.LastInstall)
	}
	if r.SystemUpdate.LastSearch.IsZero() {
		fmt.Printf("%-40s |  %s \n", "Last updates search date", "Unknown")
	} else {
		fmt.Printf("%-40s |  %v \n", "Last updates search date", r.SystemUpdate.LastSearch)
	}
	fmt.Printf("%-40s |  %t \n", "Pending updates", r.SystemUpdate.PendingUpdates)
}

func (r *Report) getWindowsUpdateStatus() error {
	automaticUpdateSettings, err := newIAutomaticUpdates()
	if err != nil {
		return err
	} else {
		r.SystemUpdate.Status = getAutomaticUpdatesStatus(automaticUpdateSettings.NotificationLevel)
	}
	return nil
}

func (r *Report) getWindowsUpdateDates() error {
	automaticUpdateResults, err := newIAutomaticUpdate2()
	if err != nil {
		return err
	}

	r.SystemUpdate.LastInstall = automaticUpdateResults.LastInstallationSuccessDate.Local()
	r.SystemUpdate.LastSearch = automaticUpdateResults.LastSearchSuccessDate.Local()
	return nil
}

func (r *Report) getPendingUpdates() error {
	// Get information about pending updates. THIS QUERY IS SLOW
	// Ref: https://github.com/ceshihao/windowsupdate/blob/master/examples/query_update_history/main.go
	// OLE Initialize
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_SPEED_OVER_MEMORY)
	if err != nil {
		return err
	}
	defer ole.CoUninitialize()
	session, err := wu.NewUpdateSession()
	if err != nil {
		return err
	}
	searcher, err := session.CreateUpdateSearcher()
	if err != nil {
		return err
	}

	// TODO There is an exception for Windows 10 (HP laptop)
	result, err := searcher.Search("IsAssigned=1 and IsHidden=0 and IsInstalled=0 and Type='Software'")
	if err != nil {
		return err
	}
	r.SystemUpdate.PendingUpdates = len(result.Updates) > 0
	return nil
}

func (r *Report) getUpdatesHistory() error {
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_SPEED_OVER_MEMORY)
	if err != nil {
		return err
	}
	defer ole.CoUninitialize()
	session, err := wu.NewUpdateSession()
	if err != nil {
		return err
	}

	searcher, err := session.CreateUpdateSearcher()
	if err != nil {
		return err
	}

	result, err := searcher.QueryHistoryAll()
	if err != nil {
		panic(err)
	}

	updates := []openuem_nats.Update{}
	for _, entry := range result {
		if entry.ClientApplicationID == "MoUpdateOrchestrator" {
			update := openuem_nats.Update{
				Title:      entry.Title,
				Date:       *entry.Date,
				SupportURL: entry.SupportUrl,
			}
			updates = append(updates, update)
		}
	}
	r.Updates = updates

	return nil
}

func getAutomaticUpdatesStatus(notificationLevel int32) string {
	switch notificationLevel {
	case int32(NOTIFICATION_LEVEL_NOT_CONFIGURED):
		return "Automatic updates are not configured"
	case int32(NOTIFICATION_LEVEL_DISABLED):
		return "Automatic updates are disabled"
	case int32(NOTIFICATION_LEVEL_NOTIFY_BEFORE_DOWNLOAD):
		return "Updates are downloaded and installed by user intervention"
	case int32(NOTIFICATION_LEVEL_NOTIFY_BEFORE_INSTALLATION):
		return "Updates are installed by user intervention"
	case int32(NOTIFICATION_LEVEL_SCHEDULED_INSTALLATION):
		return "Updates are installed automatically"
	}
	return "Unknown"
}

// IAutomaticUpdatesResult contains the read-only properties that describe Automatic Updates.
// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iautomaticupdatesresults
type IAutomaticUpdatesResults struct {
	disp                        *ole.IDispatch
	LastInstallationSuccessDate *time.Time
	LastSearchSuccessDate       *time.Time
}

type IAutomaticUpdatesSettings struct {
	disp              *ole.IDispatch
	NotificationLevel int32
}

func toIAutomaticUpdatesSettings(iAutomaticUpdatesSettingsDisp *ole.IDispatch) (*IAutomaticUpdatesSettings, error) {
	var err error
	iAutomaticUpdatesSettings := &IAutomaticUpdatesSettings{
		disp: iAutomaticUpdatesSettingsDisp,
	}

	if iAutomaticUpdatesSettings.NotificationLevel, err = toInt32Err(oleutil.GetProperty(iAutomaticUpdatesSettingsDisp, "NotificationLevel")); err != nil {
		return nil, err
	}

	return iAutomaticUpdatesSettings, nil
}

func toIAutomaticUpdates(IAutomaticUpdatesDisp *ole.IDispatch) (*IAutomaticUpdatesSettings, error) {
	settingsDisp, err := toIDispatchErr(oleutil.GetProperty(IAutomaticUpdatesDisp, "Settings"))
	if err != nil {
		return nil, err
	}
	return toIAutomaticUpdatesSettings(settingsDisp)
}

func toIAutomaticUpdates2(IAutomaticUpdates2Disp *ole.IDispatch) (*IAutomaticUpdatesResults, error) {
	resultsDisp, err := toIDispatchErr(oleutil.GetProperty(IAutomaticUpdates2Disp, "Results"))
	if err != nil {
		return nil, err
	}
	return toIAutomaticUpdatesResults(resultsDisp)
}

func toIAutomaticUpdatesResults(iAutomaticUpdatesResultsDisp *ole.IDispatch) (*IAutomaticUpdatesResults, error) {
	var err error
	iAutomaticUpdatesResults := &IAutomaticUpdatesResults{
		disp: iAutomaticUpdatesResultsDisp,
	}

	if iAutomaticUpdatesResults.LastInstallationSuccessDate, err = toTimeErr(oleutil.GetProperty(iAutomaticUpdatesResultsDisp, "LastInstallationSuccessDate")); err != nil {
		return nil, err
	}

	if iAutomaticUpdatesResults.LastSearchSuccessDate, err = toTimeErr(oleutil.GetProperty(iAutomaticUpdatesResultsDisp, "LastSearchSuccessDate")); err != nil {
		return nil, err
	}

	return iAutomaticUpdatesResults, nil
}

// NewIAutomaticUpdate2 creates a new IAutomaticUpdates2 interface.
// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iautomaticupdates2
func newIAutomaticUpdate2() (*IAutomaticUpdatesResults, error) {
	unknown, err := oleutil.CreateObject("Microsoft.Update.AutoUpdate")
	if err != nil {
		return nil, err
	}

	// Ref: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-uamg/e839e7e0-1795-451b-94ef-abacd6cbecac
	iid_iautomaticupdates2 := ole.NewGUID("4A2F5C31-CFD9-410E-B7FB-29A653973A0F")
	disp, err := unknown.QueryInterface(iid_iautomaticupdates2)
	if err != nil {
		return nil, err
	}
	return toIAutomaticUpdates2(disp)
}

// NewIAutomaticUpdatesSettings creates a new IAutomaticUpdatesSettings interface.
// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iautomaticupdates2
func newIAutomaticUpdates() (*IAutomaticUpdatesSettings, error) {
	unknown, err := oleutil.CreateObject("Microsoft.Update.AutoUpdate")
	if err != nil {
		return nil, err
	}

	// Ref: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-uamg/e839e7e0-1795-451b-94ef-abacd6cbecac
	iidIAutomaticUpdates := ole.NewGUID("673425BF-C082-4C7C-BDFD-569464B8E0CE")
	disp, err := unknown.QueryInterface(iidIAutomaticUpdates)
	if err != nil {
		return nil, err
	}
	return toIAutomaticUpdates(disp)
}

func toIDispatchErr(result *ole.VARIANT, err error) (*ole.IDispatch, error) {
	if err != nil {
		return nil, err
	}
	return variantToIDispatch(result), nil
}

func variantToIDispatch(v *ole.VARIANT) *ole.IDispatch {
	value := v.Value()
	if value == nil {
		return nil
	}
	return v.ToIDispatch()
}

func toTimeErr(result *ole.VARIANT, err error) (*time.Time, error) {
	if err != nil {
		return nil, err
	}
	return variantToTime(result), nil
}

func variantToTime(v *ole.VARIANT) *time.Time {
	value := v.Value()
	if value == nil {
		return nil
	}
	valueTime := value.(time.Time)
	return &valueTime
}

func toInt32Err(result *ole.VARIANT, err error) (int32, error) {
	if err != nil {
		return 0, err
	}
	return variantToInt32(result), nil
}

func variantToInt32(v *ole.VARIANT) int32 {
	value := v.Value()
	if value == nil {
		return 0
	}
	return value.(int32)
}
