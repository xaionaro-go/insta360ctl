package messagecode

import "fmt"

// Code represents a command/response/notification code for the Insta360 BLE/WiFi protocol.
//
// These values come from the official MessageCode protobuf enum in libOne.so.
// The same codes are used over both BLE (Architecture B) and WiFi (TCP port 6666).
//
// Response packets use HTTP-like status codes (200=OK, 400=bad request, 500=error, 501=not implemented).
type Code uint16

// Phone → Camera commands (0x00-0xFF range).
const (
	CodeBegin                     Code = 0  // 0x00
	CodeStartLiveStream           Code = 1  // 0x01
	CodeStopLiveStream            Code = 2  // 0x02
	CodeTakePicture               Code = 3  // 0x03 — verified on GO 3
	CodeStartCapture              Code = 4  // 0x04 — verified on GO 3
	CodeStopCapture               Code = 5  // 0x05 — verified on GO 3
	CodeCancelCapture             Code = 6  // 0x06
	CodeSetOptions                Code = 7  // 0x07
	CodeGetOptions                Code = 8  // 0x08
	CodeSetPhotographyOptions     Code = 9  // 0x09
	CodeGetPhotographyOptions     Code = 10 // 0x0A
	CodeGetFileExtra              Code = 11 // 0x0B
	CodeDeleteFiles               Code = 12 // 0x0C
	CodeGetFileList               Code = 13 // 0x0D
	CodeTakePictureWithoutStoring Code = 14 // 0x0E
	CodeGetCurrentCaptureStatus   Code = 15 // 0x0F
	CodeSetFileExtra              Code = 16 // 0x10
	CodeGetTimelapseOptions       Code = 17 // 0x11
	CodeSetTimelapseOptions       Code = 18 // 0x12
	CodeGetGyro                   Code = 19 // 0x13
	CodeStartTimelapse            Code = 22 // 0x16 — verified on GO 3
	CodeStopTimelapse             Code = 23 // 0x17 — verified on GO 3
	CodeEraseSDCard               Code = 24 // 0x18
	CodeCalibrateGyro             Code = 25 // 0x19 — official: CALIBRATE_GYRO, but GO 3 returns battery info here
	CodeScanBTPeripheral          Code = 26 // 0x1A
	CodeConnectToBTPeripheral     Code = 27 // 0x1B
	CodeDisconnectBTPeripheral    Code = 28 // 0x1C
	CodeGetConnectedBTPeripherals Code = 29 // 0x1D
	CodeGetMiniThumbnail          Code = 30 // 0x1E
	CodeTestSDCardSpeed           Code = 31 // 0x1F
	CodeRebootCamera              Code = 32 // 0x20
	CodeOpenCameraWifi            Code = 33 // 0x21
	CodeCloseCameraWifi           Code = 34 // 0x22
	CodeOpenIperf                 Code = 35 // 0x23
	CodeCloseIperf                Code = 36 // 0x24
	CodeGetIperfAverage           Code = 37 // 0x25
	CodeGetFileInfoList           Code = 38 // 0x26
	CodeCheckAuthorization        Code = 39 // 0x27 — verified on GO 3
	CodeCancelAuthorization       Code = 40 // 0x28
	CodeStartBulletTimeCapture    Code = 41 // 0x29
	CodeSetSubmodeOptions         Code = 42 // 0x2A
	CodeGetSubmodeOptions         Code = 43 // 0x2B
	CodeStopBulletTimeCapture     Code = 48 // 0x30
	CodeOpenOLED                  Code = 49 // 0x31
	CodeCloseOLED                 Code = 50 // 0x32
	CodeStartHDRCapture           Code = 51 // 0x33
	CodeStopHDRCapture            Code = 52 // 0x34
	CodeUploadGPS                 Code = 53 // 0x35 — verified on GO 3
	CodeSetSyncCaptureMode        Code = 54 // 0x36
	CodeGetSyncCaptureMode        Code = 55 // 0x37
	CodeSetStandbyMode            Code = 56 // 0x38
	CodeRestoreFactorySettings    Code = 57 // 0x39
	CodeSetTempOptionsSwitch      Code = 58 // 0x3A
	CodeGetTempOptionsSwitch      Code = 59 // 0x3B
	CodeSetKeyTimePoint           Code = 60 // 0x3C — verified on GO 3 (highlight marker)
	CodeStartTimeshiftCapture     Code = 61 // 0x3D
	CodeStopTimeshiftCapture      Code = 62 // 0x3E
	CodeSetFlowstateEnable        Code = 63 // 0x3F
	CodeGetFlowstateEnable        Code = 64 // 0x40
	CodeSetActiveSensor           Code = 65 // 0x41
	CodeGetActiveSensor           Code = 66 // 0x42
	CodeSetMultiPhotographyOpts   Code = 67 // 0x43
	CodeGetMultiPhotographyOpts   Code = 68 // 0x44
	CodeGetRecordingFile          Code = 71 // 0x47

	CodePrepareGetFilePackage       Code = 83  // 0x53
	CodeGetFilePackageFinish        Code = 84  // 0x54
	CodeSetWifiSeizeEnable          Code = 85  // 0x55
	CodeRequestAuthorization        Code = 86  // 0x56 — verified on GO 3
	CodeCancelRequestAuthorization  Code = 87  // 0x57
	CodeSetButtonPressParam         Code = 103 // 0x67
	CodeGetButtonPressParam         Code = 104 // 0x68
	CodeIFrameRequest               Code = 105 // 0x69
	CodeSetWifiConnectionInfo       Code = 112 // 0x70 — the REAL WiFi join command
	CodeGetWifiConnectionInfo       Code = 113 // 0x71 — the REAL WiFi query command
	CodeSetAccessCameraFileState    Code = 118 // 0x76
	CodeSetAppID                    Code = 120 // 0x78
	CodeResetWifi                   Code = 125 // 0x7D
	CodeStopUSBCardBackup           Code = 133 // 0x85
	CodeSetCameraLiveInfo           Code = 135 // 0x87
	CodeGetCameraLiveInfo           Code = 136 // 0x88
	CodeStartCameraLive             Code = 137 // 0x89
	CodeStopCameraLive              Code = 144 // 0x90
	CodeStartCameraLiveRecord       Code = 145 // 0x91
	CodeStopCameraLiveRecord        Code = 146 // 0x92
	CodeSetWifiMode                 Code = 147 // 0x93
	CodeGetWifiScanList             Code = 148 // 0x94
	CodeGetConnectedWifiList        Code = 149 // 0x95
	CodeGetWifiMode                 Code = 150 // 0x96
	CodePrepareGetFileSyncPackage   Code = 151 // 0x97
	CodeGetFilePackageSyncFinish    Code = 152 // 0x98
	CodeDarkEISStatus               Code = 157 // 0x9D
	CodeGetCloudStorageUploadStatus Code = 160 // 0xA0
	CodeSetCloudStorageUploadStatus Code = 161 // 0xA1
	CodeGetCloudStorageBindStatus   Code = 162 // 0xA2
	CodeSetCloudStorageBindStatus   Code = 163 // 0xA3
	CodePauseRecording              Code = 164 // 0xA4
	CodeNotifyOTAError              Code = 167 // 0xA7
	CodeGetDownloadFileList         Code = 172 // 0xAC
	CodeDownloadInfo                Code = 173 // 0xAD
	CodeDelWifiHistoryInfo          Code = 175 // 0xAF
	CodeSetFavorite                 Code = 176 // 0xB0
	CodeQuickreaderGetStatus        Code = 182 // 0xB6
	CodeAddDownloadListResultSync   Code = 190 // 0xBE
	CodeGetEditInfoList             Code = 201 // 0xC9
)

// Phone → Camera request range.
const (
	CodePhoneRequestBegin Code = 4096 // 0x1000
)

// Camera → Phone notification codes (8192+ range).
const (
	CodeNotifyFirmwareUpgradeComplete       Code = 8193 // 0x2001
	CodeNotifyCaptureAutoSplit              Code = 8194 // 0x2002
	CodeNotifyBatteryUpdate                 Code = 8195 // 0x2003
	CodeNotifyBatteryLow                    Code = 8196 // 0x2004
	CodeNotifyShutdown                      Code = 8197 // 0x2005
	CodeNotifyStorageUpdate                 Code = 8198 // 0x2006
	CodeNotifyStorageFull                   Code = 8199 // 0x2007
	CodeNotifyKeyPressed                    Code = 8200 // 0x2008
	CodeNotifyCaptureStopped                Code = 8201 // 0x2009
	CodeNotifyTakePictureStateUpdate        Code = 8202 // 0x200A
	CodeNotifyDeleteFilesProgress           Code = 8203 // 0x200B
	CodeNotifyPhoneInsert                   Code = 8204 // 0x200C
	CodeNotifyBTDiscoverPeripheral          Code = 8205 // 0x200D
	CodeNotifyBTConnectedToPeripheral       Code = 8206 // 0x200E
	CodeNotifyBTDisconnectedPeripheral      Code = 8207 // 0x200F
	CodeNotifyCurrentCaptureStatus          Code = 8208 // 0x2010
	CodeNotifyAuthorizationResult           Code = 8209 // 0x2011
	CodeNotifyTimelapseStatusUpdate         Code = 8210 // 0x2012
	CodeNotifySyncCaptureModeUpdate         Code = 8211 // 0x2013
	CodeNotifySyncCaptureButtonTrigger      Code = 8212 // 0x2014
	CodeNotifyBTRemoteVerUpdated            Code = 8213 // 0x2015
	CodeNotifyCamTemperatureValue           Code = 8214 // 0x2016
	CodeNotifyCamWifiStart                  Code = 8215 // 0x2017
	CodeNotifyCamBTMsgAnalyzeFailed         Code = 8216 // 0x2018
	CodeNotifyChargeBoxBatteryUpdate        Code = 8217 // 0x2019
	CodeNotifyLiveviewBeginRotate           Code = 8219 // 0x201B
	CodeNotifyExposureUpdate                Code = 8220 // 0x201C
	CodeNotifyChargeBoxConnectStatus        Code = 8222 // 0x201E
	CodeNotifyWifiStatus                    Code = 8232 // 0x2028 — WiFi connection result
	CodeNotifyUpdateLiveStreamParams        Code = 8234 // 0x202A
	CodeNotifyFirmwareUpgradeStatusToApp    Code = 8238 // 0x202E
	CodeNotifyUSBCardStatus                 Code = 8242 // 0x2032
	CodeNotifyCameraLiveStatus              Code = 8246 // 0x2036
	CodeNotifyWifiModeChange                Code = 8247 // 0x2037
	CodeNotifyDataExportStatus              Code = 8248 // 0x2038
	CodeNotifyWifiScanListChanged           Code = 8249 // 0x2039
	CodeNotifyDetectedFace                  Code = 8250 // 0x203A
	CodeNotifyDarkEISStatus                 Code = 8252 // 0x203C
	CodeNotifyCloudStorageBindStatus        Code = 8255 // 0x203F
	CodeNotifySupportTakePhotoOnRecStatus   Code = 8256 // 0x2040
	CodeNotifyNeedDownloadFile              Code = 8259 // 0x2043
	CodeNotifyIntervalRecInfo               Code = 8270 // 0x204E
	CodeNotifyCamSubmodeChange              Code = 8275 // 0x2053
	CodeNotifyDeleteFileResult              Code = 8279 // 0x2057
	CodeNotifyFavoriteChangeStatus          Code = 8284 // 0x205C
	CodeNotifyUserTakeover                  Code = 8285 // 0x205D
)

// Factory command range.
const (
	CodeFactoryBegin Code = 12288 // 0x3000
)

// Response status codes (used in the command code field of response packets).
const (
	CodeResponseOK         Code = 0x00C8 // 200 = success
	CodeResponseBadRequest Code = 0x0190 // 400 = unknown/bad command
	CodeResponseError      Code = 0x01F4 // 500 = execution error
	CodeResponseNotImpl    Code = 0x01F5 // 501 = not implemented
)

// Backward-compatible aliases for callers using old names.
const (
	// Commands verified working on GO 3.
	CodeTakePhoto      = CodeTakePicture     // 0x03
	CodeStartRecording = CodeStartCapture    // 0x04
	CodeStopRecording  = CodeStopCapture     // 0x05
	CodeSetHighlight   = CodeSetKeyTimePoint // 0x3C
	CodeSetGPS         = CodeUploadGPS       // 0x35
	CodeSetHDR         = CodeStartHDRCapture // 0x33
	CodeSetTimelapse   = CodeStartTimelapse  // 0x16

	// GO 3 quirk: command 0x19 (officially CALIBRATE_GYRO) returns battery data on GO 3.
	// This is a firmware-level deviation from the official protocol.
	CodeGo3GetBattery = CodeCalibrateGyro // 0x19
)

// GO 3 observed notification codes (from real camera traffic).
// These are the decimal equivalents of the hex values observed on the wire.
// Some overlap with official enum names at the same numeric values.
const (
	CodeGo2NotifyCaptureState  Code = 0x2006 // 8198 — overlaps NotifyStorageUpdate
	CodeGo2NotifyDeviceInfo    Code = 0x200A // 8202 — overlaps NotifyTakePictureStateUpdate
	CodeGo2NotifyStorageState  Code = 0x2010 // 8208 — overlaps NotifyCurrentCaptureStatus
	CodeGo2NotifyBatteryState  Code = 0x2021 // 8225 — no official overlap
	CodeGo2NotifyPowerState    Code = 0x2025 // 8229 — no official overlap
	CodeGo2NotifyPeriodicStatus Code = 0x2026 // 8230 — no official overlap
)

// Legacy aliases for callers using old code names.
// These point to the official enum values at the same numeric code.
// The GO 3 firmware may interpret these codes differently from the official names.
const (
	CodeSetCaptureMode Code = 0x0C // Official: DeleteFiles — but GO 3 uses for mode setting
	CodeGetStorageInfo Code = 0x10 // Official: SetFileExtra — GO 3 returns 500
	CodeGetBatteryInfo Code = 0x12 // Official: SetTimelapseOptions — X3 may use for battery
	CodeGetDeviceInfo  Code = 0x28 // Official: CancelAuthorization — some models use for device info
	CodeGetCameraState Code = 0x3D // Official: StartTimeshiftCapture — some models use for state
	CodePowerOff       Code = 0x37 // Official: GetSyncCaptureMode — some models use for power off
)

func (c Code) String() string {
	switch c {
	case CodeBegin:
		return "Begin"
	case CodeStartLiveStream:
		return "StartLiveStream"
	case CodeStopLiveStream:
		return "StopLiveStream"
	case CodeTakePicture:
		return "TakePicture"
	case CodeStartCapture:
		return "StartCapture"
	case CodeStopCapture:
		return "StopCapture"
	case CodeCancelCapture:
		return "CancelCapture"
	case CodeSetOptions:
		return "SetOptions"
	case CodeGetOptions:
		return "GetOptions"
	case CodeSetPhotographyOptions:
		return "SetPhotographyOptions"
	case CodeGetPhotographyOptions:
		return "GetPhotographyOptions"
	case CodeGetFileExtra:
		return "GetFileExtra"
	case CodeDeleteFiles:
		return "DeleteFiles"
	case CodeGetFileList:
		return "GetFileList"
	case CodeTakePictureWithoutStoring:
		return "TakePictureWithoutStoring"
	case CodeGetCurrentCaptureStatus:
		return "GetCurrentCaptureStatus"
	case CodeSetFileExtra:
		return "SetFileExtra"
	case CodeGetTimelapseOptions:
		return "GetTimelapseOptions"
	case CodeSetTimelapseOptions:
		return "SetTimelapseOptions"
	case CodeGetGyro:
		return "GetGyro"
	case CodeStartTimelapse:
		return "StartTimelapse"
	case CodeStopTimelapse:
		return "StopTimelapse"
	case CodeEraseSDCard:
		return "EraseSDCard"
	case CodeCalibrateGyro:
		return "CalibrateGyro/Go3:GetBattery"
	case CodeScanBTPeripheral:
		return "ScanBTPeripheral"
	case CodeConnectToBTPeripheral:
		return "ConnectToBTPeripheral"
	case CodeDisconnectBTPeripheral:
		return "DisconnectBTPeripheral"
	case CodeGetConnectedBTPeripherals:
		return "GetConnectedBTPeripherals"
	case CodeGetMiniThumbnail:
		return "GetMiniThumbnail"
	case CodeTestSDCardSpeed:
		return "TestSDCardSpeed"
	case CodeRebootCamera:
		return "RebootCamera"
	case CodeOpenCameraWifi:
		return "OpenCameraWifi"
	case CodeCloseCameraWifi:
		return "CloseCameraWifi"
	case CodeOpenIperf:
		return "OpenIperf"
	case CodeCloseIperf:
		return "CloseIperf"
	case CodeGetIperfAverage:
		return "GetIperfAverage"
	case CodeGetFileInfoList:
		return "GetFileInfoList"
	case CodeCheckAuthorization:
		return "CheckAuthorization"
	case CodeCancelAuthorization:
		return "CancelAuthorization"
	case CodeStartBulletTimeCapture:
		return "StartBulletTimeCapture"
	case CodeSetSubmodeOptions:
		return "SetSubmodeOptions"
	case CodeGetSubmodeOptions:
		return "GetSubmodeOptions"
	case CodeStopBulletTimeCapture:
		return "StopBulletTimeCapture"
	case CodeOpenOLED:
		return "OpenOLED"
	case CodeCloseOLED:
		return "CloseOLED"
	case CodeStartHDRCapture:
		return "StartHDRCapture"
	case CodeStopHDRCapture:
		return "StopHDRCapture"
	case CodeUploadGPS:
		return "UploadGPS"
	case CodeSetSyncCaptureMode:
		return "SetSyncCaptureMode"
	case CodeGetSyncCaptureMode:
		return "GetSyncCaptureMode"
	case CodeSetStandbyMode:
		return "SetStandbyMode"
	case CodeRestoreFactorySettings:
		return "RestoreFactorySettings"
	case CodeSetTempOptionsSwitch:
		return "SetTempOptionsSwitch"
	case CodeGetTempOptionsSwitch:
		return "GetTempOptionsSwitch"
	case CodeSetKeyTimePoint:
		return "SetKeyTimePoint"
	case CodeStartTimeshiftCapture:
		return "StartTimeshiftCapture"
	case CodeStopTimeshiftCapture:
		return "StopTimeshiftCapture"
	case CodeSetFlowstateEnable:
		return "SetFlowstateEnable"
	case CodeGetFlowstateEnable:
		return "GetFlowstateEnable"
	case CodeSetActiveSensor:
		return "SetActiveSensor"
	case CodeGetActiveSensor:
		return "GetActiveSensor"
	case CodeSetMultiPhotographyOpts:
		return "SetMultiPhotographyOptions"
	case CodeGetMultiPhotographyOpts:
		return "GetMultiPhotographyOptions"
	case CodeGetRecordingFile:
		return "GetRecordingFile"
	case CodePrepareGetFilePackage:
		return "PrepareGetFilePackage"
	case CodeGetFilePackageFinish:
		return "GetFilePackageFinish"
	case CodeSetWifiSeizeEnable:
		return "SetWifiSeizeEnable"
	case CodeRequestAuthorization:
		return "RequestAuthorization"
	case CodeCancelRequestAuthorization:
		return "CancelRequestAuthorization"
	case CodeSetButtonPressParam:
		return "SetButtonPressParam"
	case CodeGetButtonPressParam:
		return "GetButtonPressParam"
	case CodeIFrameRequest:
		return "IFrameRequest"
	case CodeSetWifiConnectionInfo:
		return "SetWifiConnectionInfo"
	case CodeGetWifiConnectionInfo:
		return "GetWifiConnectionInfo"
	case CodeSetAccessCameraFileState:
		return "SetAccessCameraFileState"
	case CodeSetAppID:
		return "SetAppID"
	case CodeResetWifi:
		return "ResetWifi"
	case CodeStopUSBCardBackup:
		return "StopUSBCardBackup"
	case CodeSetCameraLiveInfo:
		return "SetCameraLiveInfo"
	case CodeGetCameraLiveInfo:
		return "GetCameraLiveInfo"
	case CodeStartCameraLive:
		return "StartCameraLive"
	case CodeStopCameraLive:
		return "StopCameraLive"
	case CodeStartCameraLiveRecord:
		return "StartCameraLiveRecord"
	case CodeStopCameraLiveRecord:
		return "StopCameraLiveRecord"
	case CodeSetWifiMode:
		return "SetWifiMode"
	case CodeGetWifiScanList:
		return "GetWifiScanList"
	case CodeGetConnectedWifiList:
		return "GetConnectedWifiList"
	case CodeGetWifiMode:
		return "GetWifiMode"
	case CodePrepareGetFileSyncPackage:
		return "PrepareGetFileSyncPackage"
	case CodeGetFilePackageSyncFinish:
		return "GetFilePackageSyncFinish"
	case CodeDarkEISStatus:
		return "DarkEISStatus"
	case CodeGetCloudStorageUploadStatus:
		return "GetCloudStorageUploadStatus"
	case CodeSetCloudStorageUploadStatus:
		return "SetCloudStorageUploadStatus"
	case CodeGetCloudStorageBindStatus:
		return "GetCloudStorageBindStatus"
	case CodeSetCloudStorageBindStatus:
		return "SetCloudStorageBindStatus"
	case CodePauseRecording:
		return "PauseRecording"
	case CodeNotifyOTAError:
		return "NotifyOTAError"
	case CodeGetDownloadFileList:
		return "GetDownloadFileList"
	case CodeDownloadInfo:
		return "DownloadInfo"
	case CodeDelWifiHistoryInfo:
		return "DelWifiHistoryInfo"
	case CodeSetFavorite:
		return "SetFavorite"
	case CodeQuickreaderGetStatus:
		return "QuickreaderGetStatus"
	case CodeAddDownloadListResultSync:
		return "AddDownloadListResultSync"
	case CodeGetEditInfoList:
		return "GetEditInfoList"

	// Notifications
	case CodeNotifyFirmwareUpgradeComplete:
		return "Notify:FirmwareUpgradeComplete"
	case CodeNotifyCaptureAutoSplit:
		return "Notify:CaptureAutoSplit"
	case CodeNotifyBatteryUpdate:
		return "Notify:BatteryUpdate"
	case CodeNotifyBatteryLow:
		return "Notify:BatteryLow"
	case CodeNotifyShutdown:
		return "Notify:Shutdown"
	case CodeNotifyStorageUpdate:
		return "Notify:StorageUpdate"
	case CodeNotifyStorageFull:
		return "Notify:StorageFull"
	case CodeNotifyKeyPressed:
		return "Notify:KeyPressed"
	case CodeNotifyCaptureStopped:
		return "Notify:CaptureStopped"
	case CodeNotifyTakePictureStateUpdate:
		return "Notify:TakePictureStateUpdate"
	case CodeNotifyDeleteFilesProgress:
		return "Notify:DeleteFilesProgress"
	case CodeNotifyPhoneInsert:
		return "Notify:PhoneInsert"
	case CodeNotifyBTDiscoverPeripheral:
		return "Notify:BTDiscoverPeripheral"
	case CodeNotifyBTConnectedToPeripheral:
		return "Notify:BTConnectedToPeripheral"
	case CodeNotifyBTDisconnectedPeripheral:
		return "Notify:BTDisconnectedPeripheral"
	case CodeNotifyCurrentCaptureStatus:
		return "Notify:CurrentCaptureStatus"
	case CodeNotifyAuthorizationResult:
		return "Notify:AuthorizationResult"
	case CodeNotifyTimelapseStatusUpdate:
		return "Notify:TimelapseStatusUpdate"
	case CodeNotifySyncCaptureModeUpdate:
		return "Notify:SyncCaptureModeUpdate"
	case CodeNotifySyncCaptureButtonTrigger:
		return "Notify:SyncCaptureButtonTrigger"
	case CodeNotifyBTRemoteVerUpdated:
		return "Notify:BTRemoteVerUpdated"
	case CodeNotifyCamTemperatureValue:
		return "Notify:CamTemperatureValue"
	case CodeNotifyCamWifiStart:
		return "Notify:CamWifiStart"
	case CodeNotifyCamBTMsgAnalyzeFailed:
		return "Notify:CamBTMsgAnalyzeFailed"
	case CodeNotifyChargeBoxBatteryUpdate:
		return "Notify:ChargeBoxBatteryUpdate"
	case CodeNotifyLiveviewBeginRotate:
		return "Notify:LiveviewBeginRotate"
	case CodeNotifyExposureUpdate:
		return "Notify:ExposureUpdate"
	case CodeNotifyChargeBoxConnectStatus:
		return "Notify:ChargeBoxConnectStatus"
	case CodeNotifyWifiStatus:
		return "Notify:WifiStatus"
	case CodeNotifyUpdateLiveStreamParams:
		return "Notify:UpdateLiveStreamParams"
	case CodeNotifyFirmwareUpgradeStatusToApp:
		return "Notify:FirmwareUpgradeStatusToApp"
	case CodeNotifyUSBCardStatus:
		return "Notify:USBCardStatus"
	case CodeNotifyCameraLiveStatus:
		return "Notify:CameraLiveStatus"
	case CodeNotifyWifiModeChange:
		return "Notify:WifiModeChange"
	case CodeNotifyDataExportStatus:
		return "Notify:DataExportStatus"
	case CodeNotifyWifiScanListChanged:
		return "Notify:WifiScanListChanged"
	case CodeNotifyDetectedFace:
		return "Notify:DetectedFace"
	case CodeNotifyDarkEISStatus:
		return "Notify:DarkEISStatus"
	case CodeNotifyCloudStorageBindStatus:
		return "Notify:CloudStorageBindStatus"
	case CodeNotifySupportTakePhotoOnRecStatus:
		return "Notify:SupportTakePhotoOnRecStatus"
	case CodeNotifyNeedDownloadFile:
		return "Notify:NeedDownloadFile"
	case CodeNotifyIntervalRecInfo:
		return "Notify:IntervalRecInfo"
	case CodeNotifyCamSubmodeChange:
		return "Notify:CamSubmodeChange"
	case CodeNotifyDeleteFileResult:
		return "Notify:DeleteFileResult"
	case CodeNotifyFavoriteChangeStatus:
		return "Notify:FavoriteChangeStatus"
	case CodeNotifyUserTakeover:
		return "Notify:UserTakeover"

	// Response status codes
	case CodeResponseOK:
		return "OK(200)"
	case CodeResponseBadRequest:
		return "BadRequest(400)"
	case CodeResponseError:
		return "Error(500)"
	case CodeResponseNotImpl:
		return "NotImplemented(501)"

	default:
		return fmt.Sprintf("Unknown(0x%04X)", uint16(c))
	}
}
