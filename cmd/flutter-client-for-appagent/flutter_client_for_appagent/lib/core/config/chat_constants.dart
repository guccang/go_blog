part of '../../main.dart';

const String _baseUrlOverrideKey = 'client_config::base_url_override';
const String _lastLoginUserIdKey = 'auth::last_user_id';
const String _refreshTokenStorageKey = 'auth::refresh_token';
const String _historyStoragePrefix = 'chat_history::';
const String _historyBackupStoragePrefix = 'chat_history_secure::';
const int _chatHistoryRetainLimit = 520;
const String _lastReadAtStoragePrefix = 'chat_last_read_at';
const String _codegenModeKey = 'codegen::last_mode';
const String _codeProjectKey = 'codegen::last_code_project';
const String _codeToolKey = 'codegen::last_code_tool';
const String _claudeSettingsKey = 'codegen::last_claude_settings';
const String _resumeLastSessionKey = 'codegen::resume_last_session';
const String _deployProjectKey = 'codegen::last_deploy_project';
const String _deployTargetKey = 'codegen::last_deploy_target';
const String _deployArgsKey = 'codegen::last_deploy_args';
const String _codeSearchKey = 'codegen::code_search';
const String _deploySearchKey = 'codegen::deploy_search';
const String _debugBundleModeKey = 'codegen::debug_bundle_mode';
const String _codegenHistoryKey = 'codegen::history';
const String _codegenHistoryBackupKey = 'codegen_secure::history';
const String _codegenHistoryLastBackupAtKey = 'codegen::history_last_backup_at';
const String _cortanaEnabledKey = 'cortana::enabled';
const String _cortanaAllowFullAccessKey = 'cortana::allow_full_access';
const String _cortanaAutoPlayKey = 'cortana::auto_play';
const String _cortanaProactiveModeKey = 'cortana::proactive_mode';
const String _cortanaHighFreqStartHourKey = 'cortana::high_freq_start_hour';
const String _cortanaHighFreqStartMinuteKey = 'cortana::high_freq_start_minute';
const String _cortanaHighFreqEndHourKey = 'cortana::high_freq_end_hour';
const String _cortanaHighFreqEndMinuteKey = 'cortana::high_freq_end_minute';
const String _cortanaPersonaNameKey = 'cortana::persona_name';
const String _cortanaOwnerTitleKey = 'cortana::owner_title';
const String _cortanaPersonaDescriptionKey = 'cortana::persona_description';
const String _cortanaVoiceWakeEnabledKey = 'cortana::voice_wake_enabled';
const String _cortanaWakePhraseKey = 'cortana::wake_phrase';
const String _cortanaLive2dModelsKey = 'cortana::live2d_models';
const String _cortanaSelectedLive2dModelKey = 'cortana::selected_live2d_model';
const String _cortanaLive2dViewTransformsKey =
    'cortana::live2d_view_transforms';
const Duration _sessionRefreshSkew = Duration(minutes: 1);
final FlutterSecureStorage _secureStorage = const FlutterSecureStorage(
  aOptions: AndroidOptions(encryptedSharedPreferences: true),
  iOptions: IOSOptions(
    accessibility: KeychainAccessibility.first_unlock_this_device,
  ),
);
const List<Duration> _sherpaDownloadRetryDelays = <Duration>[
  Duration(seconds: 1),
  Duration(seconds: 2),
  Duration(seconds: 4),
];

const Duration _streamFlushInterval = Duration(milliseconds: 80);
const int _codegenStreamSegmentLimit = 2048;
const int _codegenProcessEntryByteLimit = 2048;
const Duration _codegenHistoryTimeout = Duration(hours: 1);
const Duration _codegenHistoryTimeoutSweepInterval = Duration(minutes: 5);

const List<String> _sherpaAsrModelUrls = <String>[
  'https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17.tar.bz2',
];
const List<String> _sherpaKwsModelUrls = <String>[
  'https://github.com/k2-fsa/sherpa-onnx/releases/download/kws-models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01-mobile.tar.bz2',
  'https://github.com/k2-fsa/sherpa-onnx/releases/download/kws-models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01.tar.bz2',
];
const String _sherpaDownloadProgressKey = 'sherpa_download_progress';
const String _sherpaDownloadBytesKey = 'sherpa_download_bytes';
const String _sherpaManualDownloadUrlKey = 'sherpa_manual_download_url';
const String _sherpaActiveDownloadUrlKey = 'sherpa_active_download_url';
const int _sherpaStartupAutoWorkMaxBytes = 512 * 1024 * 1024;
