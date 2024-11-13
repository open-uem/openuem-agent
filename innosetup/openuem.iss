; Script generated by the Inno Setup Script Wizard.
; SEE THE DOCUMENTATION FOR DETAILS ON CREATING INNO SETUP SCRIPT FILES!

#define MyAppName "OpenUEM Agent"
#define MyAppVersion "0.1.0"
#define MyAppPublisher "Miguel Angel Alvarez Cabrerizo"
#define MyAppURL "https://github.com/doncicuto/openuem-agent"
#define MyAppExeName "openuem-agent.exe"

[Setup]
; NOTE: The value of AppId uniquely identifies this application. Do not use the same AppId value in installers for other applications.
; (To generate a new GUID, click Tools | Generate GUID inside the IDE.)
AppId={{A28A7369-DE9F-4D22-80C4-FD2C9425F194}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
LicenseFile=license.txt
DisableDirPage=yes
DisableProgramGroupPage=yes
DirExistsWarning=no
PrivilegesRequired=admin
OutputBaseFilename=openuem-agent-setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
WizardImageFile=openuem_icon.bmp
WizardSmallImageFile=openuem_small_icon.bmp
UninstallDisplayIcon="{app}\assets\openuem.ico"

[Languages]
Name: "english"; MessagesFile: "Languages\English.isl"
Name: "spanish"; MessagesFile: "Languages\Spanish.isl"

[Files]
Source: "C:\Users\mcabr\go\src\github.com\doncicuto\openuem-agent\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "C:\Users\mcabr\go\src\github.com\doncicuto\openuem-message\openuem_message.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "C:\Users\mcabr\go\src\github.com\doncicuto\openuem-updater-service\windows\openuem-updater-service.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "C:\Users\mcabr\go\src\github.com\doncicuto\openuem-agent\innosetup\assets\*"; DestDir: "{app}\assets"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{src}\ca.cer"; DestDir: "{app}\certificates"; Flags: external ignoreversion
Source: "{src}\agent.cer"; DestDir: "{app}\certificates"; Flags: external ignoreversion
Source: "{src}\agent.key"; DestDir: "{app}\certificates"; Flags: external ignoreversion
Source: "{src}\console.cer"; DestDir: "{app}\certificates"; Flags: external ignoreversion

[Registry]
Root: HKLM; Subkey: "Software\OpenUEM"; Flags:uninsdeletekeyifempty
Root: HKLM; Subkey: "Software\OpenUEM\Agent"; Flags:uninsdeletekeyifempty
Root: HKLM; Subkey: "Software\OpenUEM\Agent"; ValueType: dword; ValueName: "Enabled"; ValueData: "1"; Flags: uninsdeletekey
Root: HKLM; Subkey: "Software\OpenUEM\Agent"; ValueType: dword; ValueName: "ExecuteTaskEveryXMinutes"; ValueData: "5"; Flags: uninsdeletekey
Root: HKLM; Subkey: "Software\OpenUEM\Agent"; ValueType: string; ValueName: "NATSServers"; ValueData: "{code:MyServerUrl}"; Flags:uninsdeletekey


[Run]
;Add firewall rules
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall add rule name=""OpenUEM Remote Assistance"" dir=in action=allow protocol=TCP localport=1443 program=""{app}\{#MyAppExeName}"""; StatusMsg: "Adding rule for port access TCP 1443"; Flags: runhidden
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall add rule name=""OpenUEM SFTP Server"" dir=in action=allow protocol=TCP localport=2022 program=""{app}\{#MyAppExeName}"""; StatusMsg: "Adding rule for port access TCP 2022"; Flags: runhidden

[Dirs]
Name: "{app}\logs";Permissions: users-modify
Name: "{app}\config";Permissions: users-modify
Name: "{app}\updater";Permissions: users-modify
Name: "{app}\badgerdb";

[UninstallDelete]
Type: filesandordirs; Name: "{app}"


[Run]
Filename: {sys}\sc.exe; Parameters: "create openuem-agent start= auto DisplayName= ""OpenUEM Agent"" binPath= ""{app}\openuem-agent.exe""" ; Flags: runhidden
Filename: {sys}\sc.exe; Parameters: "description openuem-agent ""OpenUEM Agent service to report inventory info""" ; Flags: runhidden
Filename: {sys}\sc.exe; Parameters: "start openuem-agent" ; Flags: runhidden
Filename: {sys}\sc.exe; Parameters: "create openuem-updater-service start= auto DisplayName= ""OpenUEM Updater Service"" binPath= ""{app}\openuem-updater-service.exe""" ; Flags: runhidden
Filename: {sys}\sc.exe; Parameters: "description openuem-updater-service ""OpenUEM service to update agents and components""" ; Flags: runhidden
Filename: {sys}\sc.exe; Parameters: "start openuem-updater-service" ; Flags: runhidden

[UninstallRun]
Filename: {sys}\sc.exe; Parameters: "stop openuem-agent" ; RunOnceId: "StopService"; Flags: runhidden
Filename: {sys}\sc.exe; Parameters: "delete openuem-agent" ; RunOnceId: "DelService"; Flags: runhidden
Filename: {sys}\sc.exe; Parameters: "stop openuem-updater-service" ; RunOnceId: "StopService"; Flags: runhidden
Filename: {sys}\sc.exe; Parameters: "delete openuem-updater-service" ; RunOnceId: "DelService"; Flags: runhidden

[Code]
var
  InputQueryWizardPage: TInputQueryWizardPage;
  

procedure InitializeWizard;
var  
  AfterID: Integer;
begin
  WizardForm.LicenseAcceptedRadio.Checked := False;
  AfterID := wpSelectTasks;
    
  InputQueryWizardPage := CreateInputQueryPage(AfterID,CustomMessage('RequiredConfiguration'),CustomMessage('ServerURL'),CustomMessage('ServerURLExample'));
  InputQueryWizardPage.Add('&NATS Server Url:', False);
  InputQueryWizardPage.Values[0] := 'localhost:4433'
  AfterID := InputQueryWizardPage.ID;   
end;

function MyServerUrl(Param: String): String;
begin
  result := InputQueryWizardPage.Values[0]; 
end;
