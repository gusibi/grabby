; Grabby — Windows 安装包（NSIS）
;
; 由 scripts/package-desktop.sh 调用：
;   makensis -DVERSION=0.2.0 -DARCH=amd64 -DSOURCE=<exe> -DICON=<ico> -DOUTFILE=<out> installer.nsi

Unicode true
!include "MUI2.nsh"

!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef ARCH
  !define ARCH "amd64"
!endif
!ifndef SOURCE
  !error "SOURCE is required"
!endif
!ifndef OUTFILE
  !define OUTFILE "Grabby-${VERSION}-Windows-${ARCH}-setup.exe"
!endif
!ifndef ICON
  !define ICON ""
!endif

Name "Grabby"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\Grabby"
InstallDirRegKey HKCU "Software\Grabby" "InstallDir"
RequestExecutionLevel user

!define MUI_ABORTWARNING
!if "${ICON}" != ""
  !define MUI_ICON "${ICON}"
  !define MUI_UNICON "${ICON}"
!endif
!define MUI_FINISHPAGE_RUN "$INSTDIR\Grabby.exe"
!define MUI_FINISHPAGE_RUN_TEXT "运行 Grabby"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Grabby"

Section "Grabby" INSTALL
  SetOutPath "$INSTDIR"
  File "/oname=Grabby.exe" "${SOURCE}"
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  CreateDirectory "$SMPROGRAMS\Grabby"
  CreateShortcut "$SMPROGRAMS\Grabby\Grabby.lnk" "$INSTDIR\Grabby.exe"
  CreateShortcut "$SMPROGRAMS\Grabby\卸载 Grabby.lnk" "$INSTDIR\Uninstall.exe"
  CreateShortcut "$DESKTOP\Grabby.lnk" "$INSTDIR\Grabby.exe"

  WriteRegStr HKCU "Software\Grabby" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "Grabby"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "Grabby"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\Grabby.exe"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" 65000
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\Grabby.exe"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"

  Delete "$SMPROGRAMS\Grabby\Grabby.lnk"
  Delete "$SMPROGRAMS\Grabby\卸载 Grabby.lnk"
  RMDir "$SMPROGRAMS\Grabby"
  Delete "$DESKTOP\Grabby.lnk"

  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\Grabby"
SectionEnd
