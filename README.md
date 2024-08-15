# ![OpenUEM Agent for Windows](https://res.cloudinary.com/dyjqffeuz/image/upload/v1722080061/banner_bedozh.png)

This repository contains the source code for the OpenUEM Agent for Windows

Information is retrieved from the Windows Operating System using queries to WMI, the Registry, APIs...

The agent reports information once a day or every time the agent is started. Also the agent reports information on user demand either by the user that clicks on the system tray menu option "Run Inventory" or by the admin that sends a request from OpenUEM console.

## Execution

Agent is started reads the OpenUEM json file and check if a report has been sent today

    // TODO - should I register the agent as a Windows service that run at start?

    // TODO - Connect with NATS server

    // TODO - Subscribe to NATS coming from OpenUEM Server

    // Have I executed today, if not run

    // TODO - Publish agent info to NATS

## More information to add

- Disk type (SSD/HDD) -> MediaType (3 HDD, 4 SSD), Model,

Get-WmiObject -Namespace ROOT\Microsoft\Windows\Storage -Class MSFT_PhysicalDisk

Reference: https://learn.microsoft.com/en-us/answers/questions/350272/detect-if-system-(windows)-drive-is-ssd-or-hdd

Win32_DiskDrive may help searching SSD in Model / Caption https://www.reddit.com/r/PowerShell/comments/8q4tbr/

- Disk has bitlocker? -> https://4sysops.com/archives/check-the-bitlocker-status-of-all-pcs-in-the-network/#rtoc-3. The query would be `Get-WmiObject -Namespace root\CIMV2\Security\MicrosoftVolumeEncryption -Class Win32_EncryptableVolume` and this would be the response

```
__GENUS                          : 2
__CLASS                          : Win32_EncryptableVolume
__SUPERCLASS                     :
__DYNASTY                        : Win32_EncryptableVolume
__RELPATH                        : Win32_EncryptableVolume.DeviceID="\\\\?\\Volume{90dc9ab6-791d-46f6-ade8-e6d3201baf52
                                   }\\"
__PROPERTY_COUNT                 : 8
__DERIVATION                     : {}
__SERVER                         : LOTHLORIEN
__NAMESPACE                      : root\CIMV2\Security\MicrosoftVolumeEncryption
__PATH                           : \\LOTHLORIEN\root\CIMV2\Security\MicrosoftVolumeEncryption:Win32_EncryptableVolume.D
                                   eviceID="\\\\?\\Volume{90dc9ab6-791d-46f6-ade8-e6d3201baf52}\\"
ConversionStatus                 : 0
DeviceID                         : \\?\Volume{90dc9ab6-791d-46f6-ade8-e6d3201baf52}\
DriveLetter                      : C:
EncryptionMethod                 : 0
IsVolumeInitializedForProtection : False
PersistentVolumeID               :
ProtectionStatus                 : 0
VolumeType                       : 0
PSComputerName                   : LOTHLORIEN

__GENUS                          : 2
__CLASS                          : Win32_EncryptableVolume
__SUPERCLASS                     :
__DYNASTY                        : Win32_EncryptableVolume
__RELPATH                        : Win32_EncryptableVolume.DeviceID="\\\\?\\Volume{0005a690-0000-0000-0000-100000000000
                                   }\\"
__PROPERTY_COUNT                 : 8
__DERIVATION                     : {}
__SERVER                         : LOTHLORIEN
__NAMESPACE                      : root\CIMV2\Security\MicrosoftVolumeEncryption
__PATH                           : \\LOTHLORIEN\root\CIMV2\Security\MicrosoftVolumeEncryption:Win32_EncryptableVolume.D
                                   eviceID="\\\\?\\Volume{0005a690-0000-0000-0000-100000000000}\\"
ConversionStatus                 : 0
DeviceID                         : \\?\Volume{0005a690-0000-0000-0000-100000000000}\
DriveLetter                      : D:
EncryptionMethod                 : 0
IsVolumeInitializedForProtection : False
PersistentVolumeID               :
ProtectionStatus                 : 0
VolumeType                       : 1
PSComputerName                   : LOTHLORIEN
```

but the problem is that we required administrator privileges to do this query

However doing this: `(New-Object -ComObject Shell.Application).NameSpace('C:').Self.ExtendedProperty('System.Volume.BitLockerProtection')` specifying the drive letter if works

Reference: https://www.reddit.com/r/PowerShell/comments/jl12ux/get_bitlocker_status_without_admin_elevation/

But we've to know how to do the query and discard that if we ran a service it still works:

https://stackoverflow.com/questions/73655353/detect-bitlocker-status-without-admin-from-a-service

Other: https://stackoverflow.com/questions/41308245/detect-bitlocker-programmatically-from-c-sharp-without-admin

- Disk manufacturer e.g Toshiba, Seagate? -> We get some information (model, not vendor) from Win32_DiskDrive
- Last logged in time
- USB devices like mouse, keyboard, scanners?

## Issues and Notes

- What happens if NATS server is stopped or there's a connection issue? It seems that if we stop the server, no new messages to a subject are received, that means that the connection has been lost.

Tests. We've set MaxReconnect to -1 so the agent should try to reconnect undefinitely. According to this: https://github.com/nats-io/nats.go/issues/448#issuecomment-475673122 and as my tests validate, messages are queued on the library buffer and sent all them, so this means that if we've a problem with NATS server and the user or the admin starts to send messages (they may think something is wrong) several "useless" messages can overflow the workers once the NATS server is back. In this situation, we may have to change from pub-sub to request-reply as https://github.com/nats-io/nats-server/issues/422 states, now if you try to make a request and nobody is interested in give a reply we get an immediate response so we may not have those useless messages.

- What happens if no subscriber is there for a message? The message is lost, which is not a problem, but how can I know as a publisher that there's no subscribers. In our case we know that we're going to run the agent every hour, but we're using a date to check if we already have sent that information. As stated before, the request-reply may help us to avoid this.

- We must place the assets folder with the executable as the icon is needed
- PartOfDomain doesn't work as expected say false although the computer is part of a windows domain. SELECT Manufacturer, Model, PartOfDomain, TotalPhysicalMemory FROM Win32_ComputerSystem
- Installation date is reporting the date that Defender has been updated so it's not the date the last quality update was installed. The pending updates however it reports if quality updates are ready to download an install, which it's fine.

## Doubts about performance

### More things that agent should perform

Right now we should only have one report for day or at least one report every time the agent starts (after a reboot or it's triggered by the user or admin) so we shouldn't see a heavy use of the database. Nevertheless, this section discuss how about the agent would track if information has changed from the last report?

We should need a sqlite db to store the information tracked by the agent.

If a monitor hasn't changed, we shouldn't send that monitor. If no monitor has changed we send no monitors section in JSON, but what happens if one changed (added or deleted?) in that case we should send all the information to overwrite it all? If an app hasn't changed, we shouldn't send that app. If no apps have changed we send no apps, the same as before.

The agent may be smarter so the AgentWorker works less and hits database less to improve performance. It's best to perform heavy operations by the agent and let the worker do lighter database tasks.

The JSON may change to something different. The agent may track things that have been added and removed.

The agent may create a list of new or updated items like monitors with a UUID for each of the item, that way we can refer to it. Then in JSON a list of removed UUIDs can be added so they can be deleted one by one. Then, how we track new or updated items? We may add a property to each item like action that have values like new, updated and deleted. If item is new, the worker will create it. If item is updated we update it. It item is deleted we delete it.

The agent may create a UUID for each app, software, and item that is new for the agent....

## Useful things found

https://github.com/iamacarpet/go-win64api
