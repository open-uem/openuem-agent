package agent

import (
	"fmt"
	"time"

	wu "github.com/ceshihao/windowsupdate"
	"github.com/doncicuto/openuem-agent/internal/log"
	"github.com/doncicuto/openuem-agent/internal/utils"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Ref: https://learn.microsoft.com/en-us/windows/win32/api/wuapi/ne-wuapi-automaticupdatesnotificationlevel
type notificationLevel int32

type SystemUpdate struct {
	Status         string    `json:"status,omitempty"`
	LastInstall    time.Time `json:"last_install,omitempty"`
	LastSearch     time.Time `json:"last_search,omitempty"`
	PendingUpdates bool      `json:"pending_updates,omitempty"`
}

const (
	NOTIFICATION_LEVEL_NOT_CONFIGURED notificationLevel = iota
	NOTIFICATION_LEVEL_DISABLED
	NOTIFICATION_LEVEL_NOTIFY_BEFORE_DOWNLOAD
	NOTIFICATION_LEVEL_NOTIFY_BEFORE_INSTALLATION
	NOTIFICATION_LEVEL_SCHEDULED_INSTALLATION
)

func (a *Agent) getSystemUpdateInfo() {
	a.Edges.SystemUpdate = SystemUpdate{}
	// Get information about Windows Update settings

	// TODO 1 (security) get information about what SMB client version is installed
	// Maybe we can query Win32_OptionalFeature and check for \\LOTHLORIEN\root\cimv2:Win32_OptionalFeature.Name="SMB1Protocol-Client" and
	// its install state as this is an optional feature

	// TODO 2 (security) check if firewall is enabled in the three possible domains

	if err := a.Edges.SystemUpdate.getWindowsUpdateStatus(); err != nil {
		log.Logger.Printf("[ERROR]: could not get windows update status info information from wuapi: %v", err)
	} else {
		log.Logger.Printf("[INFO]: windows update status info has been retrieved from wuapi")
	}

	if err := a.Edges.SystemUpdate.getWindowsUpdateDates(); err != nil {
		log.Logger.Printf("[ERROR]: could not get windows update dates information from wuapi: %v", err)
	} else {
		log.Logger.Printf("[INFO]: windows update dates info has been retrieved from wuapi")
	}

	if err := a.Edges.SystemUpdate.getPendingUpdates(); err != nil {
		log.Logger.Printf("[ERROR]: could not get pending updates information from wuapi: %v", err)
	} else {
		log.Logger.Printf("[INFO]: pending updates info has been retrieved from wuapi")
	}
}

func (a *Agent) logSystemUpdate() {
	fmt.Printf("\n** 🔄 Updates *******************************************************************************************************\n")
	fmt.Printf("%-40s |  %s \n", "Automatic Updates status", a.Edges.SystemUpdate.Status)
	if a.Edges.SystemUpdate.LastInstall.IsZero() {
		fmt.Printf("%-40s |  %s \n", "Last updates installation date", "Unknown")
	} else {
		fmt.Printf("%-40s |  %v \n", "Last updates installation date", a.Edges.SystemUpdate.LastInstall)
	}
	if a.Edges.SystemUpdate.LastSearch.IsZero() {
		fmt.Printf("%-40s |  %s \n", "Last updates search date", "Unknown")
	} else {
		fmt.Printf("%-40s |  %v \n", "Last updates search date", a.Edges.SystemUpdate.LastSearch)
	}
	fmt.Printf("%-40s |  %t \n", "Pending updates", a.Edges.SystemUpdate.PendingUpdates)
}

func (mySystemUpdate *SystemUpdate) getWindowsUpdateStatus() error {
	automaticUpdateSettings, err := newIAutomaticUpdates()
	if err != nil {
		return err
	} else {
		mySystemUpdate.Status = getAutomaticUpdatesStatus(automaticUpdateSettings.NotificationLevel)
	}
	return nil
}

func (mySystemUpdate *SystemUpdate) getWindowsUpdateDates() error {
	automaticUpdateResults, err := newIAutomaticUpdate2()
	if err != nil {
		return err
	}

	mySystemUpdate.LastInstall = automaticUpdateResults.LastInstallationSuccessDate.Local()
	mySystemUpdate.LastSearch = automaticUpdateResults.LastSearchSuccessDate.Local()
	return nil
}

func (mySystemUpdate *SystemUpdate) getPendingUpdates() error {
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

	result, err := searcher.Search("IsAssigned=1 and IsHidden=0 and IsInstalled=0 and Type='Software'")
	if err != nil {
		return err
	}
	mySystemUpdate.PendingUpdates = len(result.Updates) > 0
	return nil
}

func getAutomaticUpdatesStatus(notificationLevel int32) string {
	switch notificationLevel {
	case int32(NOTIFICATION_LEVEL_NOT_CONFIGURED):
		return "Automatic updates are not configured"
	case int32(NOTIFICATION_LEVEL_DISABLED):
		return "Automatic updates are disabled"
	case int32(NOTIFICATION_LEVEL_NOTIFY_BEFORE_DOWNLOAD):
		return "Updates are downloaded and installed by user intervention "
	case int32(NOTIFICATION_LEVEL_NOTIFY_BEFORE_INSTALLATION):
		return "Updates are installed by user intervention "
	case int32(NOTIFICATION_LEVEL_SCHEDULED_INSTALLATION):
		return "Updates are installed automatically "
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

	if iAutomaticUpdatesSettings.NotificationLevel, err = utils.ToInt32Err(oleutil.GetProperty(iAutomaticUpdatesSettingsDisp, "NotificationLevel")); err != nil {
		return nil, err
	}

	return iAutomaticUpdatesSettings, nil
}

func toIAutomaticUpdates(IAutomaticUpdatesDisp *ole.IDispatch) (*IAutomaticUpdatesSettings, error) {
	settingsDisp, err := utils.ToIDispatchErr(oleutil.GetProperty(IAutomaticUpdatesDisp, "Settings"))
	if err != nil {
		return nil, err
	}
	return toIAutomaticUpdatesSettings(settingsDisp)
}

func toIAutomaticUpdates2(IAutomaticUpdates2Disp *ole.IDispatch) (*IAutomaticUpdatesResults, error) {
	resultsDisp, err := utils.ToIDispatchErr(oleutil.GetProperty(IAutomaticUpdates2Disp, "Results"))
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

	if iAutomaticUpdatesResults.LastInstallationSuccessDate, err = utils.ToTimeErr(oleutil.GetProperty(iAutomaticUpdatesResultsDisp, "LastInstallationSuccessDate")); err != nil {
		return nil, err
	}

	if iAutomaticUpdatesResults.LastSearchSuccessDate, err = utils.ToTimeErr(oleutil.GetProperty(iAutomaticUpdatesResultsDisp, "LastSearchSuccessDate")); err != nil {
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
