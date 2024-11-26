package report

import (
	"context"
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

func (r *Report) getSystemUpdateInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: system updates info has been requested")
	}

	// Get information about Windows Update settings

	// TODO 1 (security) get information about what SMB client version is installed
	// Maybe we can query Win32_OptionalFeature and check for \\LOTHLORIEN\root\cimv2:Win32_OptionalFeature.Name="SMB1Protocol-Client" and
	// its install state as this is an optional feature

	// TODO 2 (security) check if firewall is enabled in the three possible domains

	if debug {
		log.Println("[DEBUG]: windows update status info has been requested")
	}
	if err := r.getWindowsUpdateStatusWithCancelContext(); err != nil {
		log.Printf("[ERROR]: could not get windows update status info information from wuapi: %v", err)
		return err
	} else {
		log.Printf("[INFO]: windows update status info has been retrieved from wuapi")
	}

	if debug {
		log.Println("[DEBUG]: windows update dates info has been requested")
	}
	if err := r.getWindowsUpdateDatesWithCancelContext(); err != nil {
		log.Printf("[ERROR]: could not get windows update dates information from wuapi: %v", err)
		return err
	} else {
		log.Printf("[INFO]: windows update dates info has been retrieved from wuapi")
	}

	if debug {
		log.Println("[DEBUG]: windows update pending updates info has been requested")
	}
	if err := r.getPendingUpdatesWithCancelContext(); err != nil {
		log.Printf("[ERROR]: could not get pending updates information from wuapi: %v", err)
		return err
	} else {
		log.Printf("[INFO]: pending updates info has been retrieved from wuapi")
	}

	if debug {
		log.Println("[DEBUG]: windows update history info has been requested")
	}

	if err := r.getUpdatesHistoryWithCancelContext(); err != nil {
		log.Printf("[ERROR]: could not get updates history information from wuapi: %v", err)
		return err
	} else {
		log.Printf("[INFO]: updates history info has been retrieved from wuapi")
	}

	return nil
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

func (r *Report) getWindowsUpdateStatusWithCancelContext() error {
	ctx := context.Background()
	if _, ok := ctx.Deadline(); !ok {
		ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- r.newIAutomaticUpdates()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}

}

func (r *Report) getWindowsUpdateDatesWithCancelContext() error {
	ctx := context.Background()
	if _, ok := ctx.Deadline(); !ok {
		ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- r.newIAutomaticUpdate2()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

func (r *Report) getPendingUpdatesWithCancelContext() error {
	ctx := context.Background()
	if _, ok := ctx.Deadline(); !ok {
		ctxTimeout, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- r.getPendingUpdates()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

func (r *Report) getPendingUpdates() error {
	// Get information about pending updates. THIS QUERY IS SLOW
	// Ref: https://github.com/ceshihao/windowsupdate/blob/master/examples/query_update_history/main.go
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

func (r *Report) getUpdatesHistoryWithCancelContext() error {
	ctx := context.Background()
	if _, ok := ctx.Deadline(); !ok {
		ctxTimeout, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- r.getUpdatesHistory()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

func (r *Report) getUpdatesHistory() error {
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
		return err
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
		return "systemupdate.not_configured"
	case int32(NOTIFICATION_LEVEL_DISABLED):
		return "systemupdate.disabled"
	case int32(NOTIFICATION_LEVEL_NOTIFY_BEFORE_DOWNLOAD):
		return "systemupdate.notify_before_download"
	case int32(NOTIFICATION_LEVEL_NOTIFY_BEFORE_INSTALLATION):
		return "systemupdate.notify_before_installation"
	case int32(NOTIFICATION_LEVEL_SCHEDULED_INSTALLATION):
		return "systemupdate.notify_scheduled_installation"
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
func (r *Report) newIAutomaticUpdate2() error {
	unknown, err := oleutil.CreateObject("Microsoft.Update.AutoUpdate")
	if err != nil {
		return err
	}

	// Ref: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-uamg/e839e7e0-1795-451b-94ef-abacd6cbecac
	iid_iautomaticupdates2 := ole.NewGUID("4A2F5C31-CFD9-410E-B7FB-29A653973A0F")
	disp, err := unknown.QueryInterface(iid_iautomaticupdates2)
	if err != nil {
		return err
	}

	results, err := toIAutomaticUpdates2(disp)
	if err != nil {
		return err
	}

	r.SystemUpdate.LastInstall = results.LastInstallationSuccessDate.Local()
	r.SystemUpdate.LastSearch = results.LastSearchSuccessDate.Local()
	return nil
}

// NewIAutomaticUpdatesSettings creates a new IAutomaticUpdatesSettings interface.
// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iautomaticupdates2
func (r *Report) newIAutomaticUpdates() error {
	unknown, err := oleutil.CreateObject("Microsoft.Update.AutoUpdate")
	if err != nil {
		return err
	}

	// Ref: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-uamg/e839e7e0-1795-451b-94ef-abacd6cbecac
	iidIAutomaticUpdates := ole.NewGUID("673425BF-C082-4C7C-BDFD-569464B8E0CE")
	disp, err := unknown.QueryInterface(iidIAutomaticUpdates)
	if err != nil {
		return err
	}

	settings, err := toIAutomaticUpdates(disp)
	if err != nil {
		return err
	}

	r.SystemUpdate.Status = getAutomaticUpdatesStatus(settings.NotificationLevel)
	return nil
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
