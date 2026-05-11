part of 'main.dart';

class ChatPage extends StatefulWidget {
  const ChatPage({
    super.key,
    required this.themePreset,
    required this.onThemePresetChanged,
  });

  final UiThemePreset themePreset;
  final ValueChanged<UiThemePreset> onThemePresetChanged;

  @override
  State<ChatPage> createState() => _ChatPageState();
}

class _CodegenStreamState {
  _CodegenStreamState({
    required this.scopeKey,
    required this.streamMessageId,
    required this.latestMessage,
  });

  String scopeKey;
  final String streamMessageId;
  ChatMessage latestMessage;
  String fullContent = '';
  String pendingDelta = '';
  bool finalSeen = false;
}

const bool _defaultCortanaEnabled = true;
const bool _defaultCortanaAllowFullAccess = true;
const bool _defaultCortanaAutoPlay = true;
const String _defaultCortanaProactiveMode = 'high';
const bool _defaultCortanaVoiceWakeEnabled = false;
const String _defaultCortanaWakePhrase = '嗨 Cortana';
const String _defaultCortanaOwnerTitle = '';

class _ChatPageState extends State<ChatPage> with WidgetsBindingObserver {
  final _userIdController = TextEditingController(text: 'demo-user');
  final _passwordController = TextEditingController();
  final _baseUrlController = TextEditingController();
  final _groupIdController = TextEditingController();
  final _messageController = TextEditingController();
  final _codegenPromptController = TextEditingController();
  final _codegenCodeSearchController = TextEditingController();
  final _codegenDeploySearchController = TextEditingController();
  final _deployArgsController = TextEditingController();
  final FocusNode _messageFocusNode = FocusNode();
  final _scrollController = ScrollController();
  final _controlsScrollController = ScrollController();
  final AudioRecorder _audioRecorder = AudioRecorder();
  final AudioPlayer _audioPlayer = AudioPlayer();
  final ImagePicker _imagePicker = ImagePicker();
  final LocalFilePicker _localFilePicker = LocalFilePicker();
  final stt.SpeechToText _speechToText = stt.SpeechToText();
  final VoskTranscriber _voskTranscriber = VoskTranscriber();
  final ApkInstaller _apkInstaller = ApkInstaller();
  final ZipExtractor _zipExtractor = ZipExtractor();
  final DeviceLocationProvider _locationProvider = DeviceLocationProvider();
  final ResumableFileDownloader _fileDownloader = const ResumableFileDownloader(
    retryDelays: _voskDownloadRetryDelays,
  );

  final Map<String, List<ChatMessage>> _historyByScope =
      <String, List<ChatMessage>>{};
  final Set<String> _loadedHistoryScopes = <String>{};
  final ValueNotifier<List<ChatMessage>> _activeMessagesNotifier =
      ValueNotifier<List<ChatMessage>>(const <ChatMessage>[]);
  final GlobalKey _activeMessageAnchorKey = GlobalKey();
  late final ScopedHistoryPersistenceCoordinator _historyPersistence =
      ScopedHistoryPersistenceCoordinator(_persistHistory);
  final List<GroupInfo> _groups = <GroupInfo>[];
  final List<CodingProjectInfo> _codingProjects = <CodingProjectInfo>[];
  final List<DeployProjectInfo> _deployProjects = <DeployProjectInfo>[];
  final ValueNotifier<List<CodegenHistoryItem>> _codegenHistoryNotifier =
      ValueNotifier<List<CodegenHistoryItem>>(const <CodegenHistoryItem>[]);
  final List<LlmDebugEvent> _llmDebugEvents = <LlmDebugEvent>[];
  final Set<String> _seenMessageIds = <String>{};
  final Set<String> _autoInstallTriggered = <String>{};
  final Set<String> _consumedCortanaReplyKeys = <String>{};
  final Set<String> _presentedCortanaReplyKeys = <String>{};
  final Set<String> _pendingCortanaRequestIds = <String>{};

  WebSocketChannel? _socket;
  StreamSubscription<dynamic>? _socketSub;
  Timer? _reconnectTimer;
  Timer? _cortanaWakeRestartTimer;
  Timer? _cortanaLocationTimer;
  Timer? _streamFlushTimer;
  Timer? _codegenTimeoutSweepTimer;
  final Map<String, _CodegenStreamState> _codegenStreamStates =
      <String, _CodegenStreamState>{};
  final Set<String> _pendingCodegenStreamIds = <String>{};
  bool _scrollToBottomScheduled = false;
  String? _activeMessageAnchorId;

  bool _connecting = false;
  bool _connected = false;
  bool _loggingIn = false;
  bool _recording = false;
  bool _speechReady = false;
  bool _systemSpeechReady = false;
  bool _useLocalVosk = false;
  bool _sending = false;
  bool _transcribingVoice = false;
  bool _voiceInputMode = false;
  String? _playingAudioKey;
  bool _autoReconnect = false;
  bool _configLoading = true;
  bool _sidebarExpanded = false;
  bool _controlsExpanded = false;
  bool _groupTabsExpanded = false;
  bool _passwordVisible = false;
  bool _codegenLoading = false;
  bool _codegenSending = false;
  bool _cortanaEnabled = _defaultCortanaEnabled;
  bool _cortanaAllowFullAccess = _defaultCortanaAllowFullAccess;
  bool _cortanaAutoPlay = _defaultCortanaAutoPlay;
  String _cortanaProactiveMode = _defaultCortanaProactiveMode;
  int _cortanaHighFreqStartHour = 9;
  int _cortanaHighFreqStartMinute = 0;
  int _cortanaHighFreqEndHour = 22;
  int _cortanaHighFreqEndMinute = 0;
  String _cortanaPersonaName = 'Cortana';
  String _cortanaOwnerTitle = _defaultCortanaOwnerTitle;
  String _cortanaPersonaDescription = '';
  bool _cortanaVoiceWakeEnabled = _defaultCortanaVoiceWakeEnabled;
  String _cortanaWakePhrase = _defaultCortanaWakePhrase;
  final TextEditingController _cortanaPersonaNameCtrl = TextEditingController();
  final TextEditingController _cortanaOwnerTitleCtrl = TextEditingController();
  final TextEditingController _cortanaPersonaDescCtrl = TextEditingController();
  final TextEditingController _cortanaWakePhraseCtrl = TextEditingController();
  final TextEditingController _cortanaLive2dUrlCtrl = TextEditingController();
  final TextEditingController _cortanaLogKeywordCtrl = TextEditingController();
  bool _cortanaChatSettingsExpanded = false;
  bool _cortanaChatLogsExpanded = false;
  bool _appStorageExpanded = false;
  bool _appStorageScanning = false;
  String _appStorageScanError = '';
  String _appStorageDeletingCategory = '';
  DateTime? _appStorageScannedAt;
  List<AppStorageUsage> _appStorageUsages = const <AppStorageUsage>[];
  bool _cortanaLive2dDownloading = false;
  double _cortanaLive2dDownloadProgress = 0.0;
  String _cortanaLive2dDownloadError = '';
  String _selectedCortanaLive2dModelId = '';
  List<CortanaLive2dModelInfo> _cortanaLive2dModels =
      <CortanaLive2dModelInfo>[];
  Map<String, CortanaModelViewTransform> _cortanaLive2dViewTransforms =
      <String, CortanaModelViewTransform>{};
  bool _codegenAutoDeploy = false;
  bool _deployPackOnly = false;
  bool _codegenDebugBundleMode = false;
  List<CodegenHistoryItem> _codegenHistory = [];
  String _activeCodegenHistoryId = '';
  int _lastSequence = 0;
  String _status = 'Idle';
  String _sessionToken = '';
  String _refreshToken = '';
  int _sessionExpiresAtMs = 0;
  String _obsAgentBaseUrl = '';
  String _currentGroupId = '';
  String _configError = '';
  Offset _recordDragOffset = Offset.zero;
  Offset? _recordDragStartGlobalPosition;
  String _speechDraft = '';
  String _codegenError = '';
  String _selectedCodeProjectQualifiedName = '';
  String _selectedCodeTool = '';
  String _selectedClaudeSettings = '';
  String _selectedDeployProjectQualifiedName = '';
  String _selectedDeployTarget = '';
  DateTime? _recordStartedAt;
  ClientConfig? _clientConfig;
  String? _downloadStatusLabel;
  int _downloadStatusPercent = -1;
  bool _voskModelDownloading = false;
  double _voskModelDownloadProgress = 0.0;
  String? _voskModelDownloadError;
  Future<bool>? _sessionRefreshFuture;
  RootTab _rootTab = RootTab.chat;
  CortanaDisplayMode _cortanaFloatingMode = CortanaDisplayMode.collapsed;
  final GlobalKey<CortanaPageState> _cortanaPageKey =
      GlobalKey<CortanaPageState>();
  bool _cortanaBadge = false;
  bool _cortanaImmersiveUiHidden = false;
  final CortanaBroadcastQueue _cortanaBroadcastQueue = CortanaBroadcastQueue();
  bool _appInForeground = true;
  CortanaReplyPayload? _pendingBackgroundCortanaBroadcast;
  String? _cortanaContextualExpression;
  CodegenLaunchMode _codegenMode = CodegenLaunchMode.code;
  bool _startupGreetingShown = false;
  bool _loginGreetingShown = false;
  bool _cortanaWakeListening = false;
  bool _cortanaWakeHandling = false;
  bool _cortanaWakePausedForLifecycle = false;
  bool _speechTransitioning = false;
  Future<void> _speechStopTail = Future<void>.value();
  String _lastSpeechStatusLog = '';
  String _lastCortanaWakeTranscript = '';
  String _lastCortanaWakeStateLog = '';
  bool _cortanaLocationUpdating = false;
  Map<String, dynamic>? _lastCortanaDeviceContext;
  DateTime? _lastCortanaLocationReportAt;
  Timer? _cortanaExpressionTimer;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _appendSystem('Loading client config...');
    unawaited(_restoreCodegenPreferences());
    unawaited(_loadCodegenHistory());
    unawaited(_loadClientConfig());
    unawaited(_restoreVoskDownloadProgress());
    unawaited(_restoreCortanaLive2dModels());
    _startCodegenTimeoutSweepTimer();
    _scheduleCortanaLocationRefresh(initial: true);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.inactive ||
        state == AppLifecycleState.hidden ||
        state == AppLifecycleState.paused ||
        state == AppLifecycleState.detached) {
      _appInForeground = false;
      _cortanaWakePausedForLifecycle = true;
      unawaited(_pauseCortanaWakeListening(cancel: true));
      unawaited(_flushHistoryToDisk());
    } else if (state == AppLifecycleState.resumed) {
      _appInForeground = true;
      _cortanaWakePausedForLifecycle = false;
      _resetCortanaImmersiveUi();
      _scheduleCortanaWakeRestart();
      _scheduleCortanaLocationRefresh(initial: false);
      _playPendingBackgroundCortanaBroadcast();
    }
  }

  Future<void> _restoreVoskDownloadProgress() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await _migrateLegacyVoskPartialArchive(prefs);
      final partFile = await _getVoskArchivePartFile();
      if (await partFile.exists()) {
        final partialBytes = await partFile.length();
        if (partialBytes <= 0) {
          await partFile.delete();
          await prefs.remove(_voskDownloadProgressKey);
          await prefs.remove(_voskDownloadBytesKey);
          return;
        }
        final savedProgress = await _getVoskDownloadProgress();
        final savedBytes = prefs.getInt(_voskDownloadBytesKey) ?? partialBytes;
        if (!mounted) return;
        setState(() {
          _voskModelDownloadProgress = savedProgress > 0 && savedProgress < 1.0
              ? savedProgress
              : 0.0;
          _status = 'Vosk 模型下载未完成（已下载 ${_formatBytes(savedBytes)}），点击继续下载按钮可继续';
        });
        _appendSystem('检测到未完成的 Vosk 模型下载，可点击继续下载');
      } else {
        await prefs.remove(_voskDownloadProgressKey);
        await prefs.remove(_voskDownloadBytesKey);
      }
    } catch (_) {
      // Ignore errors during progress restoration
    }
  }

  void _syncActiveMessages() {
    if (!mounted) return;
    _activeMessagesNotifier.value = _messages;
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    unawaited(_flushHistoryToDisk());
    _reconnectTimer?.cancel();
    _cortanaExpressionTimer?.cancel();
    _cortanaWakeRestartTimer?.cancel();
    _cortanaLocationTimer?.cancel();
    _streamFlushTimer?.cancel();
    _codegenTimeoutSweepTimer?.cancel();
    _activeMessagesNotifier.dispose();
    unawaited(_socketSub?.cancel());
    unawaited(_socket?.sink.close());
    unawaited(_pauseCortanaWakeListening(cancel: true));
    _userIdController.dispose();
    _passwordController.dispose();
    _baseUrlController.dispose();
    _groupIdController.dispose();
    _messageController.dispose();
    _codegenPromptController.dispose();
    _codegenCodeSearchController.dispose();
    _codegenDeploySearchController.dispose();
    _deployArgsController.dispose();
    _codegenHistoryNotifier.dispose();
    _cortanaPersonaNameCtrl.dispose();
    _cortanaOwnerTitleCtrl.dispose();
    _cortanaPersonaDescCtrl.dispose();
    _cortanaWakePhraseCtrl.dispose();
    _cortanaLive2dUrlCtrl.dispose();
    _cortanaLogKeywordCtrl.dispose();
    _messageFocusNode.dispose();
    _scrollController.dispose();
    _controlsScrollController.dispose();
    unawaited(_audioPlayer.dispose());
    unawaited(_audioRecorder.dispose());
    super.dispose();
  }

  Future<void> _flushHistoryToDisk() async {
    try {
      await _historyPersistence.flushAll(_historyByScope.keys);
    } catch (err) {
      debugPrint('Flush history failed: $err');
    }
  }

  bool _isSpeechDoneStatus(String status) {
    final normalized = status.toLowerCase().trim();
    return normalized == 'done' ||
        normalized == 'notlistening' ||
        normalized == 'not_listening';
  }

  void _appendCortanaWakeLog(String message, {bool alsoSystem = false}) {
    final text = message.trim();
    if (text.isEmpty) {
      return;
    }
    final line = '[语音唤醒] $text';
    addFlutterClientLog(line);
    debugPrint(line);
    if (alsoSystem) {
      _appendSystem(line);
    }
  }

  void _appendCortanaWakeStateLog(String message) {
    final text = message.trim();
    if (text.isEmpty || text == _lastCortanaWakeStateLog) {
      return;
    }
    _lastCortanaWakeStateLog = text;
    _appendCortanaWakeLog(text);
  }

  String _cortanaWakeBlockedReason() {
    if (!mounted) return '页面未挂载';
    if (!_cortanaEnabled) return 'Cortana 未启用';
    if (!_cortanaVoiceWakeEnabled) return '语音唤醒未启用';
    if (_cortanaWakePausedForLifecycle) return '应用不在前台';
    if (_recording) return '正在录音';
    if (_sending) return '正在发送消息';
    if (_transcribingVoice) return '正在转写语音';
    if (_cortanaWakeHandling) return '正在处理上一次唤醒';
    if (_speechTransitioning) return '语音识别正在切换状态';
    return '';
  }

  void _showCortanaWakeGreeting(String text) {
    if (!mounted) {
      return;
    }
    final messenger = ScaffoldMessenger.maybeOf(context);
    messenger
      ?..hideCurrentSnackBar()
      ..showSnackBar(
        SnackBar(content: Text(text), duration: const Duration(seconds: 2)),
      );
  }

  void _handleSpeechRecognitionStatus(String status) {
    if (_lastSpeechStatusLog != status) {
      _lastSpeechStatusLog = status;
      _appendSystem('Speech recognition status: $status');
      if (_cortanaVoiceWakeEnabled ||
          _cortanaWakeListening ||
          _cortanaWakeHandling) {
        _appendCortanaWakeLog(
          'Speech recognition status: $status, '
          'listening=$_cortanaWakeListening, handling=$_cortanaWakeHandling',
        );
      }
    }
    if (_isSpeechDoneStatus(status) && _cortanaWakeListening) {
      _cortanaWakeListening = false;
      unawaited(_handleCortanaWakeSessionEnded());
    }
  }

  void _handleSpeechRecognitionError(Object error) {
    _appendSystem('Speech recognition error: $error');
    _appendCortanaWakeLog('Speech recognition error: $error');
    if (_isSpeechBusyError(error)) {
      unawaited(_resetBusySpeechRecognition());
      return;
    }
    if (_cortanaWakeListening) {
      _cortanaWakeListening = false;
      if (!_cortanaWakeHandling) {
        _scheduleCortanaWakeRestart(delay: const Duration(seconds: 2));
      }
    }
  }

  bool _isSpeechBusyError(Object error) {
    final text = error.toString().toLowerCase();
    return text.contains('error_busy') || text.contains('recognizer_busy');
  }

  Future<void> _resetBusySpeechRecognition() async {
    _cortanaWakeListening = false;
    _appendCortanaWakeLog('系统语音识别忙，重置后重启监听');
    await _stopSpeechRecognition(cancel: true);
    if (!_cortanaWakeHandling) {
      _scheduleCortanaWakeRestart(delay: const Duration(seconds: 2));
    }
  }

  Future<void> _stopSpeechRecognition({required bool cancel}) {
    final previous = _speechStopTail;
    late final Future<void> next;
    next = previous.catchError((_) {}).then((_) async {
      _speechTransitioning = true;
      try {
        if (cancel) {
          await _speechToText.cancel();
        } else if (_speechToText.isListening) {
          await _speechToText.stop();
        }
      } catch (_) {
        try {
          await _speechToText.cancel();
        } catch (_) {}
      }
      await Future<void>.delayed(const Duration(milliseconds: 900));
      _speechTransitioning = false;
    });
    _speechStopTail = next;
    return next;
  }

  Future<void> _handleCortanaWakeSessionEnded() async {
    _appendCortanaWakeLog('监听会话结束，准备重启');
    await _stopSpeechRecognition(cancel: true);
    if (!_cortanaWakeHandling) {
      _scheduleCortanaWakeRestart(delay: const Duration(milliseconds: 1200));
    }
  }

  Future<bool> _ensureSystemSpeechRecognitionReady({
    bool silent = false,
  }) async {
    if (_systemSpeechReady) {
      return true;
    }
    try {
      _appendCortanaWakeLog('初始化系统语音识别');
      final available = await _speechToText.initialize(
        onError: _handleSpeechRecognitionError,
        onStatus: _handleSpeechRecognitionStatus,
      );
      if (!mounted) {
        return false;
      }
      if (!available && !silent) {
        _appendSystem('Speech recognition not available on this device.');
      }
      setState(() {
        _systemSpeechReady = available;
      });
      _appendCortanaWakeLog('系统语音识别初始化结果: available=$available');
      return available;
    } catch (err, stack) {
      if (!mounted) {
        return false;
      }
      if (!silent) {
        _appendSystem('Speech recognition init failed: $err');
      }
      _appendCortanaWakeLog('系统语音识别初始化失败: $err');
      debugPrint('Speech init error: $err\n$stack');
      setState(() {
        _systemSpeechReady = false;
      });
      return false;
    }
  }

  Future<void> _initVoice() async {
    final config = _clientConfig;
    final prefs = await SharedPreferences.getInstance();
    if (_isAndroidHost && config != null && config.enableLocalVosk) {
      final modelPath = await _resolveAvailableVoskModelPath(
        preferredPath: config.voskModelPath,
      );
      if (modelPath != null) {
        final localModelPath = await _getLocalVoskModelPath();
        final savedModelPath = prefs.getString('vosk_model_path')?.trim() ?? '';
        final localModelPrefix = '$localModelPath${Platform.pathSeparator}';
        if ((modelPath == localModelPath ||
                modelPath.startsWith(localModelPrefix)) &&
            savedModelPath != modelPath) {
          await prefs.setString('vosk_model_path', modelPath);
        }
        try {
          final error = await _voskTranscriber.initialize(modelPath);
          if (!mounted) {
            return;
          }
          if (error == null) {
            setState(() {
              _speechReady = true;
              _useLocalVosk = true;
            });
            _appendSystem('Vosk local speech recognition is ready.');
            await _ensureSystemSpeechRecognitionReady(silent: true);
            _scheduleCortanaWakeRestart();
            return;
          }
          await prefs.remove('vosk_model_path');
          if (!mounted) {
            return;
          }
          setState(() {
            _speechReady = false;
            _useLocalVosk = false;
          });
          _appendSystem(
            'Vosk model invalid, cleared model path. Please re-download: $error',
          );
        } catch (err) {
          await prefs.remove('vosk_model_path');
          if (!mounted) {
            return;
          }
          setState(() {
            _speechReady = false;
            _useLocalVosk = false;
          });
          _appendSystem(
            'Initialize Vosk failed, cleared model path. Please re-download: $err',
          );
        }
      } else if ((config.voskModelPath).trim().isNotEmpty) {
        await prefs.remove('vosk_model_path');
        _appendSystem(
          'Vosk model directory is incomplete, fallback to system speech recognition.',
        );
      }
    }

    final available = await _ensureSystemSpeechRecognitionReady();
    if (!mounted) {
      return;
    }
    setState(() {
      _speechReady = available;
      _useLocalVosk = false;
    });
    _scheduleCortanaWakeRestart();
  }

  bool get _canListenForCortanaWake =>
      mounted &&
      _cortanaEnabled &&
      _cortanaVoiceWakeEnabled &&
      !_cortanaWakePausedForLifecycle &&
      !_recording &&
      !_sending &&
      !_transcribingVoice &&
      !_cortanaWakeHandling &&
      !_speechTransitioning;

  void _scheduleCortanaWakeRestart({
    Duration delay = const Duration(milliseconds: 500),
  }) {
    _cortanaWakeRestartTimer?.cancel();
    if (!_canListenForCortanaWake) {
      final reason = _cortanaWakeBlockedReason();
      if (reason.isNotEmpty) {
        _appendCortanaWakeStateLog('监听未启动: $reason');
      }
      return;
    }
    _appendCortanaWakeStateLog(
      '将在 ${delay.inMilliseconds}ms 后启动监听，唤醒词="$_cortanaWakePhrase"',
    );
    _cortanaWakeRestartTimer = Timer(delay, () {
      if (!mounted || !_canListenForCortanaWake) {
        final reason = _cortanaWakeBlockedReason();
        if (reason.isNotEmpty) {
          _appendCortanaWakeStateLog('定时启动取消: $reason');
        }
        return;
      }
      unawaited(_startCortanaWakeListening());
    });
  }

  Future<void> _pauseCortanaWakeListening({required bool cancel}) async {
    _cortanaWakeRestartTimer?.cancel();
    _cortanaWakeListening = false;
    _appendCortanaWakeLog('暂停监听: cancel=$cancel');
    await _stopSpeechRecognition(cancel: cancel);
  }

  Future<String?> _resolveSpeechLocaleId() async {
    try {
      final locales = await _speechToText.locales();
      if (locales.isEmpty) {
        return null;
      }
      final zhLocale = locales.firstWhere(
        (locale) =>
            locale.localeId == 'zh_CN' || locale.localeId.startsWith('zh'),
        orElse: () => locales.first,
      );
      return zhLocale.localeId;
    } catch (_) {
      return null;
    }
  }

  String _compactWakeText(String text) {
    return normalizeSpeechTranscript(
      text,
    ).toLowerCase().replaceAll(RegExp(r"""[\s,，.。!！?？、:：;；"“”'‘’]"""), '');
  }

  Set<String> _wakePhraseAliases() {
    return <String>{
      _cortanaWakePhrase,
      _defaultCortanaWakePhrase,
      '嘿 Cortana',
      'Hey Cortana',
      'Hi Cortana',
      '你好 Cortana',
      '嗨 科塔娜',
      '嘿 科塔娜',
      '嗨 小娜',
    }.map(_compactWakeText).where((text) => text.isNotEmpty).toSet();
  }

  String _stripWakePhrase(String transcript) {
    var command = normalizeSpeechTranscript(transcript);
    final configured = _cortanaWakePhrase.trim();
    if (configured.isNotEmpty) {
      command = command.replaceFirst(
        RegExp(RegExp.escape(configured), caseSensitive: false),
        '',
      );
    }
    command = command.replaceFirst(
      RegExp(
        r'(嗨|嘿|hello|hey|hi|你好)?\s*(cortana|科塔娜|小娜)\s*[,，。.!！?？、]?\s*',
        caseSensitive: false,
      ),
      '',
    );
    return normalizeSpeechTranscript(command);
  }

  String? _extractCortanaWakeCommand(String transcript) {
    final compact = _compactWakeText(transcript);
    if (compact.isEmpty) {
      return null;
    }
    for (final alias in _wakePhraseAliases()) {
      if (compact.contains(alias)) {
        return _stripWakePhrase(transcript);
      }
    }
    return null;
  }

  Future<void> _startCortanaWakeListening() async {
    if (!_canListenForCortanaWake) {
      final reason = _cortanaWakeBlockedReason();
      if (reason.isNotEmpty) {
        _appendCortanaWakeStateLog('监听未启动: $reason');
      }
      return;
    }
    if (_speechToText.isListening) {
      _appendCortanaWakeStateLog('监听未启动: 系统语音识别已在监听');
      return;
    }
    await _speechStopTail.catchError((_) {});
    final ready = await _ensureSystemSpeechRecognitionReady(silent: true);
    if (!ready || !_canListenForCortanaWake) {
      _appendCortanaWakeStateLog('监听未启动: 系统语音识别未就绪');
      return;
    }
    final hasPermission = await _audioRecorder.hasPermission();
    if (!hasPermission) {
      _appendSystem('语音唤醒需要麦克风权限。');
      _appendCortanaWakeLog('监听未启动: 缺少麦克风权限');
      return;
    }

    try {
      final localeId = await _resolveSpeechLocaleId();
      _appendCortanaWakeLog(
        '开始监听，locale=${localeId ?? 'default'}, 唤醒词="$_cortanaWakePhrase"',
      );
      _lastCortanaWakeTranscript = '';
      // speech_to_text 7.3.0's listen() has no return statement,
      // so the Future resolves to null. Set optimistic flag before
      // listen() rather than relying on isListening afterward, because
      // isListening can be out of sync with the native recognizer.
      _cortanaWakeListening = true;
      await _speechToText.listen(
        onResult: (result) {
          final transcript = normalizeSpeechTranscript(result.recognizedWords);
          if (transcript.isEmpty ||
              transcript == _lastCortanaWakeTranscript ||
              _cortanaWakeHandling) {
            return;
          }
          _lastCortanaWakeTranscript = transcript;
          _appendCortanaWakeLog(
            '识别到: "$transcript", final=${result.finalResult}',
          );
          final command = _extractCortanaWakeCommand(transcript);
          if (command != null) {
            _appendCortanaWakeLog('匹配唤醒词，初始指令="$command"');
            unawaited(_handleCortanaWakeDetected(command));
          } else if (result.finalResult) {
            _appendCortanaWakeLog('最终结果未命中唤醒词: "$transcript"');
          }
        },
        listenFor: const Duration(minutes: 5),
        pauseFor: const Duration(seconds: 3),
        localeId: localeId,
        listenOptions: stt.SpeechListenOptions(
          listenMode: stt.ListenMode.dictation,
          partialResults: true,
          cancelOnError: false,
        ),
      );
      if (mounted) {
        setState(() {
          _status = '语音唤醒监听中';
        });
      }
      _appendCortanaWakeLog(
        'listen 返回: isListening=$_speechToText.isListening',
      );
    } catch (err, stack) {
      _cortanaWakeListening = false;
      _appendCortanaWakeLog('启动监听失败: $err');
      debugPrint('Cortana wake listen error: $err\n$stack');
      // Always cancel the native recognizer before restarting, even for
      // non-busy errors (e.g. TypeError), because listen() may have already
      // started the native session.
      if (!_isSpeechBusyError(err)) {
        await _stopSpeechRecognition(cancel: true);
      }
      _scheduleCortanaWakeRestart(delay: const Duration(seconds: 2));
    }
  }

  Future<String> _listenForCortanaWakeCommand() async {
    if (!await _ensureSystemSpeechRecognitionReady(silent: true)) {
      return '';
    }
    final completer = Completer<String>();
    var transcript = '';
    try {
      await _speechStopTail.catchError((_) {});
      final localeId = await _resolveSpeechLocaleId();
      var started = await _listenForCortanaCommandOnce(
        localeId: localeId,
        onTranscript: (text, finalResult) {
          transcript = text;
          if (finalResult && text.isNotEmpty && !completer.isCompleted) {
            completer.complete(text);
          }
        },
      );
      if (!started) {
        await _stopSpeechRecognition(cancel: true);
        started = await _listenForCortanaCommandOnce(
          localeId: localeId,
          onTranscript: (text, finalResult) {
            transcript = text;
            if (finalResult && text.isNotEmpty && !completer.isCompleted) {
              completer.complete(text);
            }
          },
        );
      }
      if (!started) {
        return '';
      }
      if (mounted) {
        setState(() {
          _status = 'Cortana 已唤醒，正在聆听...';
        });
      }
      _appendCortanaWakeLog('唤醒后继续聆听指令');
      return await Future.any<String>([
        completer.future,
        Future<String>.delayed(const Duration(seconds: 10), () => transcript),
      ]);
    } catch (err, stack) {
      _appendCortanaWakeLog('唤醒后聆听指令失败: $err');
      debugPrint('Cortana command listen error: $err\n$stack');
      if (_isSpeechBusyError(err)) {
        await _stopSpeechRecognition(cancel: true);
      }
      return transcript;
    } finally {
      await _stopSpeechRecognition(cancel: false);
    }
  }

  Future<bool> _listenForCortanaCommandOnce({
    required String? localeId,
    required void Function(String transcript, bool finalResult) onTranscript,
  }) async {
    // speech_to_text 7.3.0's listen() has no return statement,
    // so the Future resolves to null. Use isListening instead.
    await _speechToText.listen(
      onResult: (result) {
        onTranscript(
          normalizeSpeechTranscript(result.recognizedWords),
          result.finalResult,
        );
      },
      listenFor: const Duration(seconds: 10),
      pauseFor: const Duration(seconds: 2),
      localeId: localeId,
      listenOptions: stt.SpeechListenOptions(
        listenMode: stt.ListenMode.dictation,
        partialResults: true,
        cancelOnError: false,
      ),
    );
    return _speechToText.isListening;
  }

  Future<void> _handleCortanaWakeDetected(String initialCommand) async {
    if (_cortanaWakeHandling) {
      _appendCortanaWakeLog('忽略重复唤醒: 正在处理上一轮');
      return;
    }
    _cortanaWakeHandling = true;
    try {
      await _pauseCortanaWakeListening(cancel: false);
      if (!mounted) {
        return;
      }
      setState(() {
        _rootTab = RootTab.cortana;
        _cortanaFloatingMode = CortanaDisplayMode.collapsed;
        _cortanaBadge = false;
        _status = 'Cortana 已唤醒';
      });
      _triggerCortanaContextualExpression('surprised');
      final greeting = normalizeSpeechTranscript(initialCommand).isEmpty
          ? '你好，我在。请继续说你的指令。'
          : '你好，我在，正在处理你的语音。';
      _showCortanaWakeGreeting(greeting);
      _appendSystem(greeting);
      _appendCortanaWakeLog('唤醒成功，唤醒词="$_cortanaWakePhrase", 提示="$greeting"');

      var command = normalizeSpeechTranscript(initialCommand);
      if (command.isEmpty) {
        command = normalizeSpeechTranscript(
          await _listenForCortanaWakeCommand(),
        );
      }
      if (command.isEmpty) {
        _appendSystem('已唤醒 Cortana，但未识别到有效语音内容。');
        _appendCortanaWakeLog('唤醒成功，但后续指令为空');
        return;
      }
      await _speakCortanaWakeCommand(command);
    } finally {
      _cortanaWakeHandling = false;
      _scheduleCortanaWakeRestart(delay: const Duration(seconds: 1));
    }
  }

  Future<void> _speakCortanaWakeCommand(String command) async {
    _appendSystem('Cortana 语音对话：$command');
    _appendCortanaWakeLog('发送唤醒语音指令: "$command"');
    for (var attempt = 0; attempt < 10; attempt++) {
      final state = _cortanaPageKey.currentState;
      if (state != null) {
        await state.speakText(command);
        return;
      }
      await Future<void>.delayed(const Duration(milliseconds: 120));
    }
    throw StateError('Cortana 页面尚未准备好。');
  }

  @override
  Widget build(BuildContext context) {
    final palette = _palette;
    final hideCortanaChrome =
        _rootTab == RootTab.cortana && _cortanaImmersiveUiHidden;
    final runningCodeCount = _runningCodegenCount(CodegenLaunchMode.code);
    final runningDeployCount = _runningCodegenCount(CodegenLaunchMode.deploy);
    final runningCodegenTotal = runningCodeCount + runningDeployCount;
    final codegenNavLabel = runningCodegenTotal > 0
        ? '编码$runningCodeCount 发布$runningDeployCount'
        : '编码发布';
    return Scaffold(
      extendBodyBehindAppBar: true,
      appBar: hideCortanaChrome
          ? null
          : AppBar(
              leading: IconButton(
                tooltip: '打开侧边栏',
                onPressed: () {
                  setState(() => _sidebarExpanded = true);
                },
                icon: const Icon(Icons.menu_rounded),
              ),
              title: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      const Text('App Agent'),
                      const SizedBox(width: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: palette.surfaceRaised.withValues(alpha: 0.9),
                          borderRadius: BorderRadius.circular(4),
                          border: Border.all(color: palette.border),
                        ),
                        child: Text(
                          appVersion,
                          style: TextStyle(
                            fontSize: 10,
                            fontWeight: FontWeight.normal,
                            color: palette.textSecondary,
                          ),
                        ),
                      ),
                    ],
                  ),
                  Text(
                    _rootTab == RootTab.chat
                        ? (_currentGroupId.isEmpty
                              ? 'Direct conversation'
                              : 'Group ${_currentGroupId.toLowerCase()}')
                        : _rootTab == RootTab.codegen
                        ? 'Fast path for /cg start and /cg deploy'
                        : _rootTab == RootTab.cortana
                        ? 'Live2D Assistant'
                        : _rootTab == RootTab.debug
                        ? 'LLM prompt and tool trace'
                        : 'App Settings',
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w500,
                      color: palette.textSecondary,
                    ),
                  ),
                ],
              ),
              actions: [
                if (_rootTab == RootTab.cortana)
                  IconButton(
                    tooltip: '播报历史',
                    onPressed: _openCortanaHistoryPage,
                    icon: const Icon(Icons.history_rounded),
                  ),
                Padding(
                  padding: const EdgeInsets.only(right: 16),
                  child: Center(
                    child: _buildStatusChip(
                      icon: Icons.wifi_tethering_rounded,
                      label: _connectionLabel,
                      color: _connectionColor,
                    ),
                  ),
                ),
              ],
            ),
      body: Stack(
        children: [
          Stack(
            children: [
              Offstage(
                offstage: _rootTab != RootTab.chat,
                child: TickerMode(
                  enabled: _rootTab == RootTab.chat,
                  child: _buildChatBody(),
                ),
              ),
              Offstage(
                offstage: _rootTab != RootTab.codegen,
                child: TickerMode(
                  enabled: _rootTab == RootTab.codegen,
                  child: _buildCodegenBody(),
                ),
              ),
              Offstage(
                offstage: _rootTab != RootTab.settings,
                child: TickerMode(
                  enabled: _rootTab == RootTab.settings,
                  child: _buildSettingsBody(),
                ),
              ),
              Offstage(
                offstage: _rootTab != RootTab.debug,
                child: TickerMode(
                  enabled: _rootTab == RootTab.debug,
                  child: _buildDebugBody(),
                ),
              ),
              _buildCortanaLayer(),
            ],
          ),
          if (_sidebarExpanded) ...[
            Positioned.fill(
              child: GestureDetector(
                onTap: () {
                  setState(() => _sidebarExpanded = false);
                },
                child: ColoredBox(color: Colors.black.withValues(alpha: 0.28)),
              ),
            ),
            Positioned(top: 0, bottom: 0, left: 0, child: _buildAppSidebar()),
          ],
        ],
      ),
      bottomNavigationBar: hideCortanaChrome || _rootTab == RootTab.settings
          ? null
          : NavigationBar(
              selectedIndex: switch (_rootTab) {
                RootTab.chat => 0,
                RootTab.codegen => 1,
                RootTab.cortana => 2,
                RootTab.debug => 3,
                RootTab.settings => 0,
              },
              onDestinationSelected: (index) {
                final nextTab = switch (index) {
                  0 => RootTab.chat,
                  1 => RootTab.codegen,
                  2 => RootTab.cortana,
                  _ => RootTab.debug,
                };
                _selectRootTab(nextTab);
              },
              destinations: [
                const NavigationDestination(
                  icon: Icon(Icons.chat_bubble_outline_rounded),
                  selectedIcon: Icon(Icons.chat_bubble_rounded),
                  label: '聊天',
                ),
                NavigationDestination(
                  icon: runningCodegenTotal > 0
                      ? Badge.count(
                          count: runningCodegenTotal,
                          child: const Icon(Icons.terminal_outlined),
                        )
                      : const Icon(Icons.terminal_outlined),
                  selectedIcon: runningCodegenTotal > 0
                      ? Badge.count(
                          count: runningCodegenTotal,
                          child: const Icon(Icons.terminal_rounded),
                        )
                      : const Icon(Icons.terminal_rounded),
                  label: codegenNavLabel,
                ),
                const NavigationDestination(
                  icon: Icon(Icons.face_outlined),
                  selectedIcon: Icon(Icons.face_rounded),
                  label: 'Cortana',
                ),
                const NavigationDestination(
                  icon: Icon(Icons.bug_report_outlined),
                  selectedIcon: Icon(Icons.bug_report_rounded),
                  label: '调试',
                ),
              ],
            ),
    );
  }
}
