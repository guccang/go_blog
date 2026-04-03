import 'dart:async';
import 'dart:convert'
    show base64Decode, base64Encode, jsonDecode, jsonEncode, utf8;
import 'dart:io';

import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:speech_to_text/speech_to_text.dart' as stt;

void main() {
  runApp(const AppAgentClientApp());
}

class AppAgentClientApp extends StatelessWidget {
  const AppAgentClientApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'App Agent Client',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        useMaterial3: true,
        colorScheme: const ColorScheme.light(
          primary: Color(0xFF154A3F),
          secondary: Color(0xFFCA8752),
          surface: Color(0xFFFFFBF5),
        ),
        scaffoldBackgroundColor: const Color(0xFFF4EFE6),
        appBarTheme: const AppBarTheme(
          backgroundColor: Colors.transparent,
          foregroundColor: Color(0xFF1D2B24),
          elevation: 0,
          centerTitle: false,
        ),
        inputDecorationTheme: InputDecorationTheme(
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(18),
            borderSide: const BorderSide(color: Color(0xFFD7CCBC)),
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(18),
            borderSide: const BorderSide(color: Color(0xFFD7CCBC)),
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(18),
            borderSide: const BorderSide(color: Color(0xFF154A3F), width: 1.4),
          ),
          filled: true,
          fillColor: const Color(0xFFFFFCF8),
          contentPadding: const EdgeInsets.symmetric(
            horizontal: 16,
            vertical: 16,
          ),
        ),
        snackBarTheme: SnackBarThemeData(
          behavior: SnackBarBehavior.floating,
          backgroundColor: const Color(0xFF21352D),
          contentTextStyle: const TextStyle(color: Colors.white),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
          ),
        ),
      ),
      home: const ChatPage(),
    );
  }
}

class ChatMessage {
  ChatMessage({
    required this.content,
    required this.direction,
    required this.timestamp,
    this.status = 'sent',
    this.scopeKey = 'direct',
    this.authorId = '',
    this.groupId = '',
    this.messageType = 'text',
    this.meta,
  });

  final String content;
  final MessageDirection direction;
  final DateTime timestamp;
  final String status;
  final String scopeKey;
  final String authorId;
  final String groupId;
  final String messageType;
  final Map<String, dynamic>? meta;

  Map<String, dynamic> toJson() {
    final sanitizedMeta = meta == null
        ? null
        : Map<String, dynamic>.from(meta!);
    sanitizedMeta?.remove('audio_base64');
    return {
      'content': content,
      'direction': direction.name,
      'timestamp': timestamp.millisecondsSinceEpoch,
      'status': status,
      'scope_key': scopeKey,
      'author_id': authorId,
      'group_id': groupId,
      'message_type': messageType,
      'meta': sanitizedMeta,
    };
  }

  factory ChatMessage.fromJson(Map<String, dynamic> json) {
    final directionName = (json['direction'] ?? 'system').toString();
    final direction = MessageDirection.values.firstWhere(
      (value) => value.name == directionName,
      orElse: () => MessageDirection.system,
    );
    return ChatMessage(
      content: (json['content'] ?? '').toString(),
      direction: direction,
      timestamp: DateTime.fromMillisecondsSinceEpoch(
        json['timestamp'] is int
            ? json['timestamp'] as int
            : int.tryParse('${json['timestamp']}') ??
                  DateTime.now().millisecondsSinceEpoch,
      ),
      status: (json['status'] ?? 'sent').toString(),
      scopeKey: (json['scope_key'] ?? 'direct').toString(),
      authorId: (json['author_id'] ?? '').toString(),
      groupId: (json['group_id'] ?? '').toString(),
      messageType: (json['message_type'] ?? 'text').toString(),
      meta: json['meta'] is Map<String, dynamic>
          ? json['meta'] as Map<String, dynamic>
          : null,
    );
  }
}

enum MessageDirection { outgoing, incoming, system }

class PushEnvelope {
  PushEnvelope({
    required this.sequence,
    required this.userId,
    required this.content,
    required this.channel,
    required this.messageType,
    required this.timestamp,
    this.meta,
  });

  factory PushEnvelope.fromJson(Map<String, dynamic> json) {
    return PushEnvelope(
      sequence: json['sequence'] is int
          ? json['sequence'] as int
          : int.tryParse('${json['sequence']}') ?? 0,
      userId: (json['user_id'] ?? '').toString(),
      content: (json['content'] ?? '').toString(),
      channel: (json['channel'] ?? '').toString(),
      messageType: (json['message_type'] ?? 'text').toString(),
      timestamp: json['timestamp'] is int
          ? json['timestamp'] as int
          : int.tryParse('${json['timestamp']}') ??
                DateTime.now().millisecondsSinceEpoch,
      meta: json['meta'] is Map<String, dynamic>
          ? json['meta'] as Map<String, dynamic>
          : null,
    );
  }

  final int sequence;
  final String userId;
  final String content;
  final String channel;
  final String messageType;
  final int timestamp;
  final Map<String, dynamic>? meta;
}

class RecordedAudio {
  const RecordedAudio({required this.path, required this.duration});

  final String path;
  final Duration duration;
}

class GroupInfo {
  const GroupInfo({
    required this.id,
    required this.members,
    required this.createdAt,
  });

  final String id;
  final List<String> members;
  final DateTime createdAt;

  factory GroupInfo.fromJson(Map<String, dynamic> json) {
    final members = (json['members'] as List<dynamic>? ?? const [])
        .map((item) => item.toString())
        .toList();
    return GroupInfo(
      id: (json['id'] ?? '').toString(),
      members: members,
      createdAt: DateTime.fromMillisecondsSinceEpoch(
        json['created_at'] is int
            ? json['created_at'] as int
            : int.tryParse('${json['created_at']}') ??
                  DateTime.now().millisecondsSinceEpoch,
      ),
    );
  }
}

class ClientConfig {
  const ClientConfig({required this.baseUrl, required this.receiveToken});

  final String baseUrl;
  final String receiveToken;

  factory ClientConfig.fromJson(Map<String, dynamic> json) {
    return ClientConfig(
      baseUrl: (json['base_url'] ?? '').toString().trim(),
      receiveToken: (json['receive_token'] ?? '').toString().trim(),
    );
  }
}

class AppAgentClient {
  static const Duration _httpTimeout = Duration(seconds: 8);
  static const Duration _wsConnectTimeout = Duration(seconds: 8);

  AppAgentClient({
    required this.baseUrl,
    required this.userId,
    required this.password,
    required this.receiveToken,
    required this.sessionToken,
  });

  final String baseUrl;
  final String userId;
  final String password;
  final String receiveToken;
  final String sessionToken;

  Future<Map<String, dynamic>> login() async {
    final uri = Uri.parse('$baseUrl/api/app/login');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json',
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
          },
          body: jsonEncode({'user_id': userId, 'password': password}),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw HttpException('login failed: ${resp.statusCode} ${resp.body}');
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  Future<void> sendAppMessage(
    String content, {
    String messageType = 'text',
    Map<String, dynamic>? meta,
  }) async {
    final uri = Uri.parse('$baseUrl/api/app/message');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json',
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
            if (sessionToken.trim().isNotEmpty)
              'X-App-Agent-Session': sessionToken.trim(),
          },
          body: jsonEncode({
            'user_id': userId,
            'content': content,
            'message_type': messageType,
            'meta': meta,
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw HttpException('send failed: ${resp.statusCode} ${resp.body}');
    }
  }

  Future<void> sendMessage(String content) => sendAppMessage(content);

  Future<List<GroupInfo>> listGroups() async {
    final uri = Uri.parse(
      '$baseUrl/api/app/groups?user_id=$userId&session_token=$sessionToken',
    );
    final resp = await http
        .get(
          uri,
          headers: {
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
            if (sessionToken.trim().isNotEmpty)
              'X-App-Agent-Session': sessionToken.trim(),
          },
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw HttpException(
        'list groups failed: ${resp.statusCode} ${resp.body}',
      );
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final groups = (data['groups'] as List<dynamic>? ?? const [])
        .map((item) => GroupInfo.fromJson(item as Map<String, dynamic>))
        .toList();
    return groups;
  }

  Future<List<GroupInfo>> mutateGroup(String action, String groupId) async {
    final uri = Uri.parse('$baseUrl/api/app/groups');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json',
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
            if (sessionToken.trim().isNotEmpty)
              'X-App-Agent-Session': sessionToken.trim(),
          },
          body: jsonEncode({
            'action': action,
            'user_id': userId,
            'group_id': groupId,
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw HttpException(
        'group $action failed: ${resp.statusCode} ${resp.body}',
      );
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final groups = (data['groups'] as List<dynamic>? ?? const [])
        .map((item) => GroupInfo.fromJson(item as Map<String, dynamic>))
        .toList();
    return groups;
  }

  Future<WebSocket> connectWebSocket() {
    final uri = _buildWsUri(baseUrl, userId, sessionToken);
    return WebSocket.connect(
      uri.toString(),
      headers: {
        if (receiveToken.trim().isNotEmpty)
          'X-App-Agent-Token': receiveToken.trim(),
        if (sessionToken.trim().isNotEmpty)
          'X-App-Agent-Session': sessionToken.trim(),
      },
    ).timeout(_wsConnectTimeout);
  }

  static Uri _buildWsUri(String baseUrl, String userId, String sessionToken) {
    final base = Uri.parse(baseUrl);
    final scheme = base.scheme == 'https' ? 'wss' : 'ws';
    final pathSegments = <String>[
      ...base.pathSegments.where((segment) => segment.isNotEmpty),
      'ws',
      'app',
    ];
    return base.replace(
      scheme: scheme,
      pathSegments: pathSegments,
      queryParameters: <String, String>{
        'user_id': userId,
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
      },
    );
  }
}

class ChatPage extends StatefulWidget {
  const ChatPage({super.key});

  @override
  State<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends State<ChatPage> {
  static const String _baseUrlOverrideKey = 'client_config::base_url_override';

  final _userIdController = TextEditingController(text: 'demo-user');
  final _passwordController = TextEditingController();
  final _baseUrlController = TextEditingController();
  final _groupIdController = TextEditingController();
  final _messageController = TextEditingController();
  final _scrollController = ScrollController();
  final AudioRecorder _audioRecorder = AudioRecorder();
  final AudioPlayer _audioPlayer = AudioPlayer();
  final ImagePicker _imagePicker = ImagePicker();
  final stt.SpeechToText _speechToText = stt.SpeechToText();

  final Map<String, List<ChatMessage>> _historyByScope =
      <String, List<ChatMessage>>{};
  final List<GroupInfo> _groups = <GroupInfo>[];

  WebSocket? _socket;
  StreamSubscription<dynamic>? _socketSub;
  Timer? _reconnectTimer;

  bool _connecting = false;
  bool _connected = false;
  bool _loggingIn = false;
  bool _recording = false;
  bool _speechReady = false;
  bool _sending = false;
  String? _playingAudioKey;
  bool _autoReconnect = false;
  bool _configLoading = true;
  bool _controlsExpanded = false;
  bool _passwordVisible = false;
  int _lastSequence = 0;
  String _status = 'Idle';
  String _sessionToken = '';
  String _currentGroupId = '';
  String _configError = '';
  Offset _recordDragOffset = Offset.zero;
  String _speechDraft = '';
  DateTime? _recordStartedAt;
  ClientConfig? _clientConfig;

  @override
  void initState() {
    super.initState();
    _appendSystem('Loading client config...');
    unawaited(_loadClientConfig());
    unawaited(_initVoice());
  }

  @override
  void dispose() {
    _reconnectTimer?.cancel();
    unawaited(_socketSub?.cancel());
    unawaited(_socket?.close());
    _userIdController.dispose();
    _passwordController.dispose();
    _baseUrlController.dispose();
    _groupIdController.dispose();
    _messageController.dispose();
    _scrollController.dispose();
    unawaited(_audioPlayer.dispose());
    unawaited(_audioRecorder.dispose());
    super.dispose();
  }

  Future<void> _initVoice() async {
    try {
      final available = await _speechToText.initialize();
      if (!mounted) {
        return;
      }
      setState(() {
        _speechReady = available;
      });
    } catch (_) {
      if (!mounted) {
        return;
      }
      setState(() {
        _speechReady = false;
      });
    }
  }

  Future<void> _loadClientConfig() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final raw = await rootBundle.loadString('assets/app_config.json');
      final assetConfig = ClientConfig.fromJson(
        jsonDecode(raw) as Map<String, dynamic>,
      );
      final savedBaseUrl = prefs.getString(_baseUrlOverrideKey)?.trim() ?? '';
      final config = ClientConfig(
        baseUrl: savedBaseUrl.isEmpty ? assetConfig.baseUrl : savedBaseUrl,
        receiveToken: assetConfig.receiveToken,
      );
      if (config.baseUrl.isEmpty) {
        throw const FormatException('base_url is required');
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _clientConfig = config;
        _baseUrlController.text = config.baseUrl;
        _configLoading = false;
        _configError = '';
        _status = 'Config loaded';
      });
      _appendSystem('Client config loaded.');
    } catch (err) {
      if (!mounted) {
        return;
      }
      setState(() {
        _clientConfig = null;
        _configLoading = false;
        _configError = 'Load client config failed: $err';
        _status = 'Config load failed';
      });
      _appendSystem(_configError);
    }
  }

  Future<void> _saveBaseUrl() async {
    final baseUrl = _baseUrlController.text.trim();
    if (baseUrl.isEmpty) {
      _appendSystem('Base URL cannot be empty.');
      return;
    }
    Uri? parsed;
    try {
      parsed = Uri.parse(baseUrl);
    } catch (_) {
      parsed = null;
    }
    if (parsed == null ||
        !parsed.hasScheme ||
        (parsed.scheme != 'http' && parsed.scheme != 'https') ||
        parsed.host.isEmpty) {
      _appendSystem('Base URL must be a valid http or https address.');
      return;
    }

    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_baseUrlOverrideKey, baseUrl);
    if (!mounted) {
      return;
    }
    setState(() {
      _clientConfig = ClientConfig(
        baseUrl: baseUrl,
        receiveToken: _clientConfig?.receiveToken ?? '',
      );
      _status = 'URL updated';
    });
    _appendSystem('Server URL updated: $baseUrl');
  }

  AppAgentClient get _client => AppAgentClient(
    baseUrl: _clientConfig?.baseUrl ?? '',
    userId: _userIdController.text.trim(),
    password: _passwordController.text,
    receiveToken: _clientConfig?.receiveToken ?? '',
    sessionToken: _sessionToken,
  );

  String get _currentScopeKey =>
      _currentGroupId.isEmpty ? 'direct' : _groupScopeKey(_currentGroupId);

  List<ChatMessage> get _messages =>
      _historyByScope[_currentScopeKey] ?? const <ChatMessage>[];

  String get _wsUrl {
    final baseUrl = _clientConfig?.baseUrl ?? '';
    final userId = _userIdController.text.trim();
    if (baseUrl.isEmpty || userId.isEmpty) {
      return 'ws://<app-agent>/ws/app?user_id=<user>';
    }
    return AppAgentClient._buildWsUri(
      baseUrl,
      userId,
      _sessionToken,
    ).toString();
  }

  Future<void> _copyText(String label, String value) async {
    if (value.trim().isEmpty) {
      return;
    }
    await Clipboard.setData(ClipboardData(text: value));
    if (!mounted) {
      return;
    }
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('$label copied'),
        duration: const Duration(seconds: 1),
      ),
    );
  }

  void _appendSystem(String text) {
    _appendMessage(
      ChatMessage(
        content: text,
        direction: MessageDirection.system,
        timestamp: DateTime.now(),
        status: 'info',
        scopeKey: _currentScopeKey,
      ),
      updateStatus: text,
    );
  }

  void _appendOutgoing(
    String text, {
    String messageType = 'text',
    Map<String, dynamic>? meta,
  }) {
    _appendMessage(
      ChatMessage(
        content: text,
        direction: MessageDirection.outgoing,
        timestamp: DateTime.now(),
        scopeKey: _currentScopeKey,
        authorId: _userIdController.text.trim(),
        groupId: _currentGroupId,
        messageType: messageType,
        meta: meta,
      ),
      updateStatus: 'Sending...',
    );
  }

  void _appendMessage(
    ChatMessage message, {
    String? updateStatus,
    bool persist = true,
  }) {
    final existing = _historyByScope[message.scopeKey] ?? <ChatMessage>[];
    _historyByScope[message.scopeKey] = <ChatMessage>[...existing, message];
    if (!mounted) {
      if (persist) {
        unawaited(_persistHistory(message.scopeKey));
      }
      return;
    }
    setState(() {
      if (updateStatus != null) {
        _status = updateStatus;
      }
    });
    if (persist) {
      unawaited(_persistHistory(message.scopeKey));
    }
    if (message.scopeKey == _currentScopeKey) {
      _jumpToBottom();
    }
  }

  void _jumpToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) {
        return;
      }
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent + 80,
        duration: const Duration(milliseconds: 180),
        curve: Curves.easeOut,
      );
    });
  }

  String _describeRequestError(Object err, {required String operation}) {
    if (err is TimeoutException) {
      return '$operation timed out. app-agent did not respond within 8 seconds.';
    }
    if (err is SocketException) {
      return '$operation failed: unable to reach app-agent.';
    }
    final raw = err.toString();
    if (raw.startsWith('HttpException: ')) {
      return '$operation failed: ${raw.substring('HttpException: '.length)}';
    }
    if (raw.startsWith('WebSocketException: ')) {
      return '$operation failed: ${raw.substring('WebSocketException: '.length)}';
    }
    return '$operation failed: $raw';
  }

  String _messagePlaybackKey(ChatMessage message) {
    return '${message.scopeKey}|${message.timestamp.microsecondsSinceEpoch}|${message.content}';
  }

  Future<void> _handleMessageTap(ChatMessage message) async {
    if (message.messageType != 'audio') {
      return;
    }
    final meta = message.meta;
    final audioPath = (meta?['audio_path'] ?? '').toString().trim();
    if (audioPath.isEmpty) {
      _appendSystem('Audio file unavailable for playback.');
      return;
    }
    final file = File(audioPath);
    if (!await file.exists()) {
      _appendSystem('Audio file not found: $audioPath');
      return;
    }

    final key = _messagePlaybackKey(message);
    try {
      if (_playingAudioKey == key) {
        await _audioPlayer.pause();
        if (mounted) {
          setState(() {
            _playingAudioKey = null;
          });
        }
        return;
      }

      await _audioPlayer.stop();
      await _audioPlayer.play(DeviceFileSource(audioPath));
      if (mounted) {
        setState(() {
          _playingAudioKey = key;
          _status = 'Playing voice message';
        });
      }
      unawaited(
        _audioPlayer.onPlayerComplete.first.then((_) {
          if (!mounted) {
            return;
          }
          setState(() {
            if (_playingAudioKey == key) {
              _playingAudioKey = null;
              _status = 'Voice playback finished';
            }
          });
        }),
      );
    } catch (err) {
      _appendSystem('Play audio failed: $err');
    }
  }

  String _groupScopeKey(String groupId) => 'group:${groupId.toLowerCase()}';

  String _historyStorageKey(String scopeKey) =>
      'chat_history::${_userIdController.text.trim()}::$scopeKey';

  Future<void> _loadHistory(String scopeKey) async {
    final userId = _userIdController.text.trim();
    if (userId.isEmpty) {
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString(_historyStorageKey(scopeKey));
    if (raw == null || raw.isEmpty) {
      _historyByScope[scopeKey] = <ChatMessage>[];
      if (mounted) {
        setState(() {});
      }
      return;
    }
    try {
      final list = (jsonDecode(raw) as List<dynamic>)
          .map((item) => ChatMessage.fromJson(item as Map<String, dynamic>))
          .toList();
      _historyByScope[scopeKey] = list;
      if (mounted) {
        setState(() {});
      }
    } catch (_) {
      _historyByScope[scopeKey] = <ChatMessage>[];
    }
  }

  Future<void> _persistHistory(String scopeKey) async {
    final userId = _userIdController.text.trim();
    if (userId.isEmpty) {
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    final history = _historyByScope[scopeKey] ?? const <ChatMessage>[];
    final data = history.map((msg) => msg.toJson()).toList();
    await prefs.setString(_historyStorageKey(scopeKey), jsonEncode(data));
  }

  Future<void> _switchToDirectScope() async {
    _currentGroupId = '';
    await _loadHistory(_currentScopeKey);
    if (mounted) {
      setState(() {});
    }
  }

  Future<void> _switchToGroupScope(String groupId) async {
    _currentGroupId = groupId;
    await _loadHistory(_currentScopeKey);
    if (mounted) {
      setState(() {});
    }
  }

  Future<void> _refreshGroups() async {
    if (_sessionToken.isEmpty) {
      return;
    }
    try {
      final groups = await _client.listGroups();
      if (mounted) {
        setState(() {
          _groups
            ..clear()
            ..addAll(groups);
          if (_currentGroupId.isNotEmpty &&
              !_groups.any((group) => group.id == _currentGroupId)) {
            _currentGroupId = '';
          }
        });
      }
      if (_currentGroupId.isNotEmpty) {
        await _loadHistory(_currentScopeKey);
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Load groups'));
    }
  }

  Future<void> _mutateGroup(String action) async {
    final groupId = _groupIdController.text.trim().toLowerCase();
    if (groupId.isEmpty) {
      _appendSystem('Group ID is required.');
      return;
    }
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    try {
      final groups = await _client.mutateGroup(action, groupId);
      if (mounted) {
        setState(() {
          _groups
            ..clear()
            ..addAll(groups);
          if (action == 'leave' && _currentGroupId == groupId) {
            _currentGroupId = '';
          } else if (action != 'leave') {
            _currentGroupId = groupId;
          }
        });
      }
      await _loadHistory(_currentScopeKey);
      _appendSystem('Group $action success: $groupId');
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Group $action'));
    }
  }

  Future<void> _login() async {
    final userId = _userIdController.text.trim();
    final password = _passwordController.text;
    if (_configLoading) {
      _appendSystem('Client config is still loading.');
      return;
    }
    if (_clientConfig == null) {
      _appendSystem(
        _configError.isEmpty ? 'Client config is unavailable.' : _configError,
      );
      return;
    }
    if (userId.isEmpty || password.isEmpty) {
      _appendSystem('User ID and password are required.');
      return;
    }
    if (_loggingIn) {
      return;
    }

    setState(() {
      _loggingIn = true;
      _status = 'Logging in...';
    });

    try {
      final resp = await _client.login();
      final sessionToken = (resp['session_token'] ?? '').toString();
      if (sessionToken.isEmpty) {
        throw const FormatException('missing session_token');
      }
      if (mounted) {
        setState(() {
          _sessionToken = sessionToken;
          _lastSequence = 0;
          _currentGroupId = '';
          _historyByScope.clear();
          _groups.clear();
          _status = 'Login success, connecting WebSocket...';
        });
      }
      await _loadHistory('direct');
      await _refreshGroups();
      _appendSystem('Login success for $userId');
      unawaited(_connectWs());
    } catch (err) {
      if (mounted) {
        setState(() {
          _sessionToken = '';
          _status = 'Login failed';
        });
      }
      _appendSystem(_describeRequestError(err, operation: 'Login'));
    } finally {
      if (mounted) {
        setState(() {
          _loggingIn = false;
        });
      }
    }
  }

  Future<void> _connectWs() async {
    final userId = _userIdController.text.trim();
    if (_clientConfig == null) {
      _appendSystem(
        _configError.isEmpty ? 'Client config is unavailable.' : _configError,
      );
      return;
    }
    if (userId.isEmpty) {
      _appendSystem('User ID is required.');
      return;
    }
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_connecting || _connected) {
      return;
    }

    _reconnectTimer?.cancel();
    setState(() {
      _connecting = true;
      _autoReconnect = true;
      _status = 'Connecting WebSocket...';
    });

    try {
      final socket = await _client.connectWebSocket();
      await _socketSub?.cancel();
      await _socket?.close();

      _socket = socket;
      _socketSub = socket.listen(
        _onWsData,
        onError: (Object err, StackTrace stackTrace) {
          _handleSocketClosed('WebSocket error: $err');
        },
        onDone: () {
          _handleSocketClosed('WebSocket disconnected');
        },
        cancelOnError: true,
      );

      if (mounted) {
        setState(() {
          _connecting = false;
          _connected = true;
          _status = 'WebSocket connected';
        });
      }
      _appendSystem('Connected: $_wsUrl');
    } catch (err) {
      if (mounted) {
        setState(() {
          _connecting = false;
          _connected = false;
          _status = 'Connect failed';
        });
      }
      _appendSystem(_describeRequestError(err, operation: 'WebSocket connect'));
      _scheduleReconnect();
    }
  }

  Future<void> _disconnectWs() async {
    _autoReconnect = false;
    _reconnectTimer?.cancel();
    await _socketSub?.cancel();
    await _socket?.close();
    _socketSub = null;
    _socket = null;
    if (mounted) {
      setState(() {
        _connecting = false;
        _connected = false;
        _status = 'WebSocket disconnected';
      });
    }
  }

  void _onWsData(dynamic data) {
    try {
      final text = data is String ? data : utf8.decode(data as List<int>);
      final decoded = jsonDecode(text) as Map<String, dynamic>;
      final envelope = PushEnvelope.fromJson(decoded);
      if (envelope.userId.isNotEmpty &&
          envelope.userId != _userIdController.text.trim()) {
        _appendSystem('Ignored message for user ${envelope.userId}');
        return;
      }
      if (envelope.sequence > 0 && envelope.sequence <= _lastSequence) {
        return;
      }
      if (envelope.sequence > 0) {
        _lastSequence = envelope.sequence;
      }

      final when = DateTime.fromMillisecondsSinceEpoch(envelope.timestamp);
      final meta = envelope.meta ?? <String, dynamic>{};
      final groupId = (meta['group_id'] ?? '').toString();
      final scopeKey = groupId.isEmpty ? 'direct' : _groupScopeKey(groupId);
      final fromUser = (meta['from_user'] ?? '').toString();
      final isSystemMessage = envelope.messageType == 'system';
      final direction = isSystemMessage
          ? MessageDirection.system
          : (groupId.isNotEmpty && fromUser == _userIdController.text.trim()
                ? MessageDirection.outgoing
                : MessageDirection.incoming);
      if (_shouldIgnoreGroupEcho(
        scopeKey: scopeKey,
        groupId: groupId,
        fromUser: fromUser,
        content: envelope.content,
        messageType: envelope.messageType,
      )) {
        return;
      }

      _appendMessage(
        ChatMessage(
          content: envelope.content,
          direction: direction,
          timestamp: when,
          scopeKey: scopeKey,
          authorId: fromUser,
          groupId: groupId,
          messageType: envelope.messageType,
          meta: meta,
        ),
        updateStatus: isSystemMessage ? envelope.content : 'Received message',
      );
    } catch (err) {
      _appendSystem('Invalid WebSocket payload: $err');
    }
  }

  void _handleSocketClosed(String text) {
    _socketSub = null;
    _socket = null;
    if (mounted) {
      setState(() {
        _connecting = false;
        _connected = false;
        _status = text;
      });
    }
    _appendSystem(text);
    _scheduleReconnect();
  }

  bool _shouldIgnoreGroupEcho({
    required String scopeKey,
    required String groupId,
    required String fromUser,
    required String content,
    required String messageType,
  }) {
    if (groupId.isEmpty || fromUser != _userIdController.text.trim()) {
      return false;
    }
    final history = _historyByScope[scopeKey];
    if (history == null || history.isEmpty) {
      return false;
    }
    final last = history.last;
    return last.direction == MessageDirection.outgoing &&
        last.groupId == groupId &&
        last.content == content &&
        last.messageType == messageType &&
        DateTime.now().difference(last.timestamp).inSeconds <= 5;
  }

  void _scheduleReconnect() {
    if (!_autoReconnect || _connecting || _connected) {
      return;
    }
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(const Duration(seconds: 3), () {
      if (_autoReconnect && !_connecting && !_connected) {
        unawaited(_connectWs());
      }
    });
  }

  bool get _cancelVoiceAction =>
      _recordDragOffset.dy < -48 && _recordDragOffset.dx < -24;

  bool get _speechVoiceAction =>
      _recordDragOffset.dy < -48 && _recordDragOffset.dx > 24;

  Future<void> _handleVoiceStart(LongPressStartDetails details) async {
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_recording || _sending) {
      return;
    }

    final hasPermission = await _audioRecorder.hasPermission();
    if (!hasPermission) {
      _appendSystem('Microphone permission denied.');
      return;
    }

    try {
      final tempDir = await getTemporaryDirectory();
      final useWindowsWave = Platform.isWindows;
      final fileExt = useWindowsWave ? 'wav' : 'm4a';
      final path =
          '${tempDir.path}${Platform.pathSeparator}app_voice_${DateTime.now().millisecondsSinceEpoch}.$fileExt';
      await _audioRecorder.start(
        RecordConfig(
          encoder: useWindowsWave ? AudioEncoder.wav : AudioEncoder.aacLc,
          bitRate: useWindowsWave ? 1411200 : 64000,
          sampleRate: useWindowsWave ? 44100 : 16000,
          numChannels: 1,
        ),
        path: path,
      );
      if (_speechReady) {
        await _speechToText.listen(
          onResult: (result) {
            if (!mounted) {
              return;
            }
            setState(() {
              _speechDraft = result.recognizedWords.trim();
            });
          },
          pauseFor: const Duration(seconds: 2),
          listenFor: const Duration(minutes: 1),
          localeId: 'zh_CN',
          listenOptions: stt.SpeechListenOptions(
            listenMode: stt.ListenMode.dictation,
            partialResults: true,
          ),
        );
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _recording = true;
        _recordDragOffset = Offset.zero;
        _speechDraft = '';
        _recordStartedAt = DateTime.now();
        _status = 'Recording...';
      });
    } catch (err) {
      _appendSystem('Voice record start failed: $err');
    }
  }

  void _handleVoiceMove(LongPressMoveUpdateDetails details) {
    if (!_recording) {
      return;
    }
    setState(() {
      _recordDragOffset = details.offsetFromOrigin;
    });
  }

  Future<void> _handleVoiceEnd(LongPressEndDetails details) async {
    if (!_recording) {
      return;
    }
    if (_cancelVoiceAction) {
      await _cancelVoice();
      return;
    }
    if (_speechVoiceAction) {
      await _sendVoiceAsText();
      return;
    }
    await _sendVoiceAsAudio();
  }

  Future<RecordedAudio?> _stopRecording({required bool discard}) async {
    final startedAt = _recordStartedAt;
    _recordStartedAt = null;
    try {
      if (_speechToText.isListening) {
        await _speechToText.stop();
      }
    } catch (_) {}

    String? path;
    try {
      path = await _audioRecorder.stop();
    } catch (_) {}

    final duration = startedAt == null
        ? Duration.zero
        : DateTime.now().difference(startedAt);

    if (!mounted) {
      return null;
    }
    setState(() {
      _recording = false;
      _recordDragOffset = Offset.zero;
    });

    if (path == null || path.isEmpty) {
      return null;
    }
    if (discard) {
      try {
        await File(path).delete();
      } catch (_) {}
      return null;
    }
    return RecordedAudio(path: path, duration: duration);
  }

  Future<void> _cancelVoice() async {
    try {
      if (_speechToText.isListening) {
        await _speechToText.cancel();
      }
    } catch (_) {}
    await _stopRecording(discard: true);
    _appendSystem('Voice input cancelled.');
  }

  Future<void> _sendVoiceAsText() async {
    final transcript = _speechDraft.trim();
    await _stopRecording(discard: true);
    if (transcript.isEmpty) {
      _appendSystem('No speech recognized. Please try again.');
      return;
    }
    _appendOutgoing('🎤 $transcript');
    setState(() {
      _sending = true;
    });
    try {
      await _client.sendAppMessage(
        transcript,
        meta: <String, dynamic>{
          'input_mode': 'voice_to_text',
          if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
          if (_currentGroupId.isNotEmpty) 'scope': 'group',
        },
      );
      if (mounted) {
        setState(() {
          _status = 'Voice text sent';
        });
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send voice text'));
    } finally {
      if (mounted) {
        setState(() {
          _sending = false;
        });
      }
    }
  }

  Future<void> _sendVoiceAsAudio() async {
    final recorded = await _stopRecording(discard: false);
    if (recorded == null) {
      _appendSystem('Voice recording unavailable.');
      return;
    }

    try {
      final file = File(recorded.path);
      final bytes = await file.readAsBytes();
      await file.delete();
      if (bytes.length > 768 * 1024) {
        _appendSystem('Voice message too large. Please keep it shorter.');
        return;
      }

      final seconds = recorded.duration.inMilliseconds / 1000;
      final label = '[Voice ${seconds.toStringAsFixed(1)}s]';
      final audioFormat = Platform.isWindows ? 'wav' : 'm4a';
      final savedAudioPath = await _persistVoiceMessage(
        bytes: bytes,
        extension: audioFormat,
      );
      _appendOutgoing(
        label,
        messageType: 'audio',
        meta: <String, dynamic>{
          'audio_path': savedAudioPath,
          'audio_format': audioFormat,
          'duration_ms': recorded.duration.inMilliseconds,
          if (_speechDraft.trim().isNotEmpty)
            'speech_text': _speechDraft.trim(),
          'input_mode': 'voice_audio',
          if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
          if (_currentGroupId.isNotEmpty) 'scope': 'group',
        },
      );
      setState(() {
        _sending = true;
      });
      try {
        await _client.sendAppMessage(
          label,
          messageType: 'audio',
          meta: <String, dynamic>{
            'audio_base64': base64Encode(bytes),
            'audio_format': audioFormat,
            'duration_ms': recorded.duration.inMilliseconds,
            if (_speechDraft.trim().isNotEmpty)
              'speech_text': _speechDraft.trim(),
            'input_mode': 'voice_audio',
            if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
            if (_currentGroupId.isNotEmpty) 'scope': 'group',
          },
        );
        if (mounted) {
          setState(() {
            _status = 'Voice audio sent';
          });
        }
      } finally {
        if (mounted) {
          setState(() {
            _sending = false;
          });
        }
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send voice audio'));
    }
  }

  Future<void> _pickAndSendImage() async {
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_sending || _recording) {
      return;
    }

    try {
      final picked = await _imagePicker.pickImage(
        source: ImageSource.gallery,
        imageQuality: 92,
      );
      if (picked == null) {
        return;
      }

      final bytes = await picked.readAsBytes();
      if (bytes.isEmpty) {
        _appendSystem('Selected image is empty.');
        return;
      }
      if (bytes.length > 4 * 1024 * 1024) {
        _appendSystem('Image too large. Please choose one under 4 MB.');
        return;
      }

      final fileName = picked.name.trim().isEmpty
          ? 'image_${DateTime.now().millisecondsSinceEpoch}.jpg'
          : picked.name.trim();
      final imageFormat = _detectImageFormat(fileName, bytes);
      final imageBase64 = base64Encode(bytes);
      final localMeta = <String, dynamic>{
        'image_base64': imageBase64,
        'image_format': imageFormat,
        'file_name': fileName,
        'input_mode': 'gallery_image',
        if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
        if (_currentGroupId.isNotEmpty) 'scope': 'group',
      };

      _appendOutgoing('', messageType: 'image', meta: localMeta);
      setState(() {
        _sending = true;
      });

      try {
        await _client.sendAppMessage('', messageType: 'image', meta: localMeta);
        if (mounted) {
          setState(() {
            _status = 'Image sent';
          });
        }
      } finally {
        if (mounted) {
          setState(() {
            _sending = false;
          });
        }
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send image'));
    }
  }

  Future<String> _persistVoiceMessage({
    required List<int> bytes,
    required String extension,
  }) async {
    final supportDir = await getApplicationSupportDirectory();
    final voiceDir = Directory(
      '${supportDir.path}${Platform.pathSeparator}voice_messages',
    );
    if (!await voiceDir.exists()) {
      await voiceDir.create(recursive: true);
    }
    final file = File(
      '${voiceDir.path}${Platform.pathSeparator}voice_${DateTime.now().millisecondsSinceEpoch}.$extension',
    );
    await file.writeAsBytes(bytes, flush: true);
    return file.path;
  }

  String _detectImageFormat(String fileName, List<int> bytes) {
    final lowerName = fileName.toLowerCase();
    if (lowerName.endsWith('.png')) {
      return 'png';
    }
    if (lowerName.endsWith('.webp')) {
      return 'webp';
    }
    if (lowerName.endsWith('.gif')) {
      return 'gif';
    }
    if (lowerName.endsWith('.bmp')) {
      return 'bmp';
    }
    if (bytes.length >= 4 &&
        bytes[0] == 0x89 &&
        bytes[1] == 0x50 &&
        bytes[2] == 0x4E &&
        bytes[3] == 0x47) {
      return 'png';
    }
    return 'jpg';
  }

  Future<void> _sendMessage() async {
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    final text = _messageController.text.trim();
    if (text.isEmpty) {
      return;
    }
    FocusScope.of(context).unfocus();
    _messageController.clear();
    _appendOutgoing(text);
    setState(() {
      _sending = true;
    });
    try {
      if (_currentGroupId.isEmpty) {
        await _client.sendMessage(text);
      } else {
        await _client.sendAppMessage(
          text,
          meta: <String, dynamic>{
            'group_id': _currentGroupId,
            'scope': 'group',
          },
        );
      }
      if (mounted) {
        setState(() {
          _status = 'Message sent';
        });
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send message'));
    } finally {
      if (mounted) {
        setState(() {
          _sending = false;
        });
      }
    }
  }

  Color get _connectionColor {
    if (_connected) {
      return const Color(0xFF187A57);
    }
    if (_connecting || _loggingIn) {
      return const Color(0xFFB9772E);
    }
    return const Color(0xFF8A5A42);
  }

  String get _connectionLabel {
    if (_connected) {
      return 'Connected';
    }
    if (_connecting) {
      return 'Connecting';
    }
    if (_sessionToken.isEmpty) {
      return 'Login required';
    }
    return 'Offline';
  }

  Widget _buildStatusChip({
    required IconData icon,
    required String label,
    required Color color,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: color.withValues(alpha: 0.18)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              color: color,
              fontWeight: FontWeight.w700,
              fontSize: 12,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildConfigItem({
    required IconData icon,
    required String label,
    required String value,
    required VoidCallback? onCopy,
  }) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFFFFFCF8),
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: const Color(0xFFE2D6C3)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: const Color(0xFF6E6253)),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  label,
                  style: const TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    color: Color(0xFF8B7A67),
                  ),
                ),
                const SizedBox(height: 4),
                SelectionArea(
                  child: Text(
                    value.isEmpty ? '-' : value,
                    style: const TextStyle(
                      fontSize: 13,
                      height: 1.35,
                      color: Color(0xFF2D241F),
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            onPressed: onCopy,
            tooltip: 'Copy $label',
            visualDensity: VisualDensity.compact,
            icon: const Icon(Icons.copy_rounded, size: 18),
          ),
        ],
      ),
    );
  }

  Widget _buildTopPanel() {
    final canLogin = !_loggingIn && !_configLoading && _clientConfig != null;
    final baseUrl = _clientConfig?.baseUrl ?? '';
    final receiveToken = _clientConfig?.receiveToken ?? '';

    return Container(
      margin: const EdgeInsets.fromLTRB(16, 0, 16, 14),
      padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [Color(0xFFFFFCF7), Color(0xFFF2E7D6)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(28),
        border: Border.all(color: const Color(0xFFE4D8C4)),
        boxShadow: const [
          BoxShadow(
            blurRadius: 24,
            color: Color(0x14000000),
            offset: Offset(0, 14),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    _buildStatusChip(
                      icon: Icons.wifi_tethering_rounded,
                      label: _connectionLabel,
                      color: _connectionColor,
                    ),
                    _buildStatusChip(
                      icon: Icons.layers_outlined,
                      label: _currentGroupId.isEmpty
                          ? 'Direct'
                          : 'Group ${_currentGroupId.toLowerCase()}',
                      color: const Color(0xFF8B633D),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              OutlinedButton.icon(
                onPressed: () {
                  setState(() {
                    _controlsExpanded = !_controlsExpanded;
                  });
                },
                style: OutlinedButton.styleFrom(
                  foregroundColor: const Color(0xFF5F4B37),
                  side: const BorderSide(color: Color(0xFFD4C7B1)),
                  minimumSize: const Size(0, 44),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                  ),
                ),
                icon: Icon(
                  _controlsExpanded
                      ? Icons.expand_less_rounded
                      : Icons.tune_rounded,
                ),
                label: Text(_controlsExpanded ? 'Hide' : 'Controls'),
              ),
            ],
          ),
          AnimatedCrossFade(
            firstChild: const SizedBox.shrink(),
            secondChild: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 14),
                _buildConfigItem(
                  icon: Icons.link_rounded,
                  label: 'Server URL',
                  value: baseUrl,
                  onCopy: baseUrl.isEmpty
                      ? null
                      : () => unawaited(_copyText('URL', baseUrl)),
                ),
                const SizedBox(height: 8),
                _buildConfigItem(
                  icon: Icons.key_rounded,
                  label: 'Receive Token',
                  value: receiveToken,
                  onCopy: receiveToken.isEmpty
                      ? null
                      : () => unawaited(_copyText('Token', receiveToken)),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _baseUrlController,
                        keyboardType: TextInputType.url,
                        decoration: const InputDecoration(
                          labelText: 'Server URL',
                          hintText: 'http://127.0.0.1:9002',
                          prefixIcon: Icon(Icons.link_rounded),
                        ),
                      ),
                    ),
                    const SizedBox(width: 10),
                    FilledButton.icon(
                      onPressed: _configLoading ? null : _saveBaseUrl,
                      style: FilledButton.styleFrom(
                        backgroundColor: const Color(0xFF8B633D),
                        foregroundColor: Colors.white,
                        minimumSize: const Size(0, 56),
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(18),
                        ),
                      ),
                      icon: const Icon(Icons.save_outlined),
                      label: const Text('Save URL'),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _userIdController,
                  decoration: const InputDecoration(
                    labelText: 'User ID',
                    hintText: 'demo-user',
                    prefixIcon: Icon(Icons.badge_outlined),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _passwordController,
                  obscureText: !_passwordVisible,
                  decoration: InputDecoration(
                    labelText: 'Password',
                    hintText: 'blog-agent password',
                    prefixIcon: const Icon(Icons.lock_outline_rounded),
                    suffixIcon: IconButton(
                      onPressed: () {
                        setState(() {
                          _passwordVisible = !_passwordVisible;
                        });
                      },
                      icon: Icon(
                        _passwordVisible
                            ? Icons.visibility_off_outlined
                            : Icons.visibility_outlined,
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                Wrap(
                  spacing: 10,
                  runSpacing: 10,
                  children: [
                    FilledButton.icon(
                      onPressed: canLogin ? _login : null,
                      style: FilledButton.styleFrom(
                        backgroundColor: const Color(0xFF154A3F),
                        foregroundColor: Colors.white,
                        minimumSize: const Size(132, 56),
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(18),
                        ),
                      ),
                      icon: Icon(
                        _sessionToken.isEmpty
                            ? Icons.login
                            : Icons.refresh_rounded,
                      ),
                      label: Text(_sessionToken.isEmpty ? 'Login' : 'Re-login'),
                    ),
                    OutlinedButton.icon(
                      onPressed: _connected || _connecting
                          ? _disconnectWs
                          : null,
                      style: OutlinedButton.styleFrom(
                        minimumSize: const Size(132, 56),
                        side: const BorderSide(color: Color(0xFFD4C7B1)),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(18),
                        ),
                      ),
                      icon: const Icon(Icons.link_off_rounded),
                      label: const Text('Disconnect'),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _groupIdController,
                  decoration: const InputDecoration(
                    labelText: 'Group ID',
                    hintText: 'party-01',
                    prefixIcon: Icon(Icons.groups_2_outlined),
                  ),
                ),
                const SizedBox(height: 12),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    FilledButton.tonal(
                      onPressed: _sessionToken.isEmpty
                          ? null
                          : () => _mutateGroup('create'),
                      child: const Text('Create'),
                    ),
                    FilledButton.tonal(
                      onPressed: _sessionToken.isEmpty
                          ? null
                          : () => _mutateGroup('join'),
                      child: const Text('Join'),
                    ),
                    OutlinedButton(
                      onPressed: _sessionToken.isEmpty
                          ? null
                          : () => _mutateGroup('leave'),
                      child: const Text('Leave'),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                Text(
                  _sessionToken.isEmpty
                      ? 'Login uses your blog-agent account. URL and token come from local JSON config.'
                      : 'Controls are collapsed by default so the chat stays primary.',
                  style: const TextStyle(
                    fontSize: 12,
                    height: 1.4,
                    color: Color(0xFF7B6D5C),
                  ),
                ),
              ],
            ),
            crossFadeState: _controlsExpanded
                ? CrossFadeState.showSecond
                : CrossFadeState.showFirst,
            duration: const Duration(milliseconds: 180),
          ),
          const SizedBox(height: 12),
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(
              children: [
                ChoiceChip(
                  selected: _currentGroupId.isEmpty,
                  label: const Text('Direct'),
                  onSelected: (_) => unawaited(_switchToDirectScope()),
                ),
                const SizedBox(width: 8),
                ..._groups.expand(
                  (group) => [
                    ChoiceChip(
                      selected: _currentGroupId == group.id,
                      label: Text('${group.id} (${group.members.length})'),
                      onSelected: (_) =>
                          unawaited(_switchToGroupScope(group.id)),
                    ),
                    const SizedBox(width: 8),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildComposer() {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 0, 16, 18),
      padding: const EdgeInsets.fromLTRB(14, 14, 14, 14),
      decoration: BoxDecoration(
        color: const Color(0xFFFFFCF8),
        borderRadius: BorderRadius.circular(26),
        border: Border.all(color: const Color(0xFFE2D6C3)),
        boxShadow: const [
          BoxShadow(
            blurRadius: 24,
            color: Color(0x12000000),
            offset: Offset(0, 10),
          ),
        ],
      ),
      child: Column(
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  _status,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Color(0xFF655848),
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          if (_recording) ...[
            _buildVoiceGestureOverlay(),
            const SizedBox(height: 12),
          ],
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              GestureDetector(
                onLongPressStart: _handleVoiceStart,
                onLongPressMoveUpdate: _handleVoiceMove,
                onLongPressEnd: _handleVoiceEnd,
                child: AnimatedContainer(
                  duration: const Duration(milliseconds: 140),
                  height: 56,
                  width: 56,
                  decoration: BoxDecoration(
                    color: _recording
                        ? const Color(0xFF9B2C2C)
                        : const Color(0xFFE6D8C2),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Icon(
                    _recording ? Icons.mic : Icons.mic_none_rounded,
                    color: _recording ? Colors.white : const Color(0xFF5A4A39),
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: TextField(
                  controller: _messageController,
                  minLines: 1,
                  maxLines: 5,
                  enabled: !_recording,
                  onSubmitted: (_) => _sendMessage(),
                  decoration: InputDecoration(
                    labelText: 'Message',
                    hintText: _currentGroupId.isEmpty
                        ? 'Ask something... or hold mic'
                        : 'Message the group directly...',
                  ),
                ),
              ),
              const SizedBox(width: 12),
              IconButton.filledTonal(
                onPressed: (_sending || _recording) ? null : _pickAndSendImage,
                style: IconButton.styleFrom(
                  minimumSize: const Size(56, 56),
                  backgroundColor: const Color(0xFFE6D8C2),
                  foregroundColor: const Color(0xFF5A4A39),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(20),
                  ),
                ),
                icon: const Icon(Icons.photo_library_rounded),
              ),
              const SizedBox(width: 12),
              FilledButton(
                onPressed: (_sending || _recording) ? null : _sendMessage,
                style: FilledButton.styleFrom(
                  backgroundColor: const Color(0xFF154A3F),
                  foregroundColor: Colors.white,
                  minimumSize: const Size(64, 56),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(20),
                  ),
                ),
                child: _sending
                    ? const SizedBox(
                        height: 18,
                        width: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.send_rounded),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildVoiceGestureOverlay() {
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        final anchor = width.clamp(220.0, 420.0);
        final centerX = 40.0;
        final bubbleSize = 70.0;
        final leftWidth = (anchor * 0.34).clamp(92.0, 132.0);
        final rightWidth = (anchor * 0.40).clamp(116.0, 168.0);
        final leftX = 0.0;
        final centerBubbleX = leftX + leftWidth + 12;
        final rightX = centerBubbleX + bubbleSize + 14;

        return Align(
          alignment: Alignment.centerLeft,
          child: SizedBox(
            width: anchor,
            height: 138,
            child: Stack(
              clipBehavior: Clip.none,
              children: [
                Positioned(
                  left: leftX,
                  bottom: 20,
                  child: _VoiceActionBubble(
                    width: leftWidth,
                    icon: Icons.close_rounded,
                    label: '左上滑取消',
                    helper: '松手即取消',
                    active: _cancelVoiceAction,
                    color: const Color(0xFFB9382F),
                    direction: _VoiceBubbleDirection.left,
                  ),
                ),
                Positioned(
                  left: rightX,
                  bottom: 20,
                  child: _VoiceActionBubble(
                    width: rightWidth,
                    icon: Icons.subtitles_rounded,
                    label: '右上滑转文字',
                    helper: '松手即发送文字',
                    active: _speechVoiceAction,
                    color: const Color(0xFF13634C),
                    direction: _VoiceBubbleDirection.right,
                  ),
                ),
                Positioned(
                  left: centerBubbleX,
                  bottom: 4,
                  child: Container(
                    width: bubbleSize,
                    height: bubbleSize,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: const Color(0xFF9B2C2C),
                      boxShadow: [
                        BoxShadow(
                          blurRadius: _cancelVoiceAction || _speechVoiceAction
                              ? 28
                              : 18,
                          color: const Color(0x559B2C2C),
                          offset: const Offset(0, 10),
                        ),
                      ],
                    ),
                    child: const Icon(Icons.mic, color: Colors.white, size: 30),
                  ),
                ),
                Positioned(
                  left: centerBubbleX - centerX,
                  top: 0,
                  child: Container(
                    width: 150,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 8,
                    ),
                    decoration: BoxDecoration(
                      color: const Color(0xFFF7E9D6),
                      borderRadius: BorderRadius.circular(16),
                      border: Border.all(color: const Color(0xFFE5C8A5)),
                    ),
                    child: Text(
                      _speechDraft.isEmpty
                          ? '按住说话，松手发送语音'
                          : '识别中：$_speechDraft',
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        color: Color(0xFF6A4A2E),
                        fontSize: 12,
                        fontWeight: FontWeight.w800,
                        height: 1.2,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      extendBodyBehindAppBar: true,
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('App Agent'),
            Text(
              _currentGroupId.isEmpty
                  ? 'Direct conversation'
                  : 'Group ${_currentGroupId.toLowerCase()}',
              style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w500),
            ),
          ],
        ),
        actions: [
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
      body: Container(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            colors: [Color(0xFFF7F0E6), Color(0xFFEDE2D0), Color(0xFFDCCDB8)],
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
          ),
        ),
        child: SafeArea(
          child: Column(
            children: [
              _buildTopPanel(),
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                  child: Container(
                    decoration: BoxDecoration(
                      color: const Color(0xFFFDF9F3).withValues(alpha: 0.9),
                      borderRadius: BorderRadius.circular(30),
                      border: Border.all(color: const Color(0xFFE2D6C3)),
                      boxShadow: const [
                        BoxShadow(
                          blurRadius: 28,
                          color: Color(0x14000000),
                          offset: Offset(0, 14),
                        ),
                      ],
                    ),
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(30),
                      child: ListView.builder(
                        controller: _scrollController,
                        padding: const EdgeInsets.fromLTRB(18, 20, 18, 20),
                        itemCount: _messages.length,
                        itemBuilder: (context, index) {
                          final msg = _messages[index];
                          return _MessageBubble(
                            message: msg,
                            isPlaying:
                                _playingAudioKey == _messagePlaybackKey(msg),
                            onTap: () => _handleMessageTap(msg),
                            onCopy: () async {
                              await Clipboard.setData(
                                ClipboardData(text: msg.content),
                              );
                              if (!context.mounted) {
                                return;
                              }
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text('Message copied'),
                                  duration: Duration(seconds: 1),
                                ),
                              );
                            },
                          );
                        },
                      ),
                    ),
                  ),
                ),
              ),
              _buildComposer(),
            ],
          ),
        ),
      ),
    );
  }
}

class _MessageBubble extends StatelessWidget {
  const _MessageBubble({
    required this.message,
    required this.onTap,
    required this.onCopy,
    this.isPlaying = false,
  });

  final ChatMessage message;
  final Future<void> Function() onTap;
  final Future<void> Function() onCopy;
  final bool isPlaying;

  @override
  Widget build(BuildContext context) {
    final isOutgoing = message.direction == MessageDirection.outgoing;
    final isSystem = message.direction == MessageDirection.system;
    final alignment = isSystem
        ? Alignment.center
        : (isOutgoing ? Alignment.centerRight : Alignment.centerLeft);
    final bgColor = isSystem
        ? const Color(0xFFE8DCC7)
        : (isOutgoing ? const Color(0xFF154A3F) : const Color(0xFFFFFCF8));
    final fgColor = isOutgoing ? Colors.white : const Color(0xFF2D241F);
    final isAudio = message.messageType == 'audio';
    final isImage = message.messageType == 'image';
    final durationMs = message.meta?['duration_ms'];
    final durationText = durationMs is num
        ? '${(durationMs / 1000).toStringAsFixed(1)}s'
        : '';
    final imageBase64 = (message.meta?['image_base64'] ?? '').toString().trim();
    Uint8List? imageBytes;
    if (isImage && imageBase64.isNotEmpty) {
      try {
        imageBytes = base64Decode(imageBase64);
      } catch (_) {
        imageBytes = null;
      }
    }

    return Align(
      alignment: alignment,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 720),
        child: InkWell(
          onTap: isAudio ? () => onTap() : null,
          onLongPress: () => onCopy(),
          borderRadius: BorderRadius.circular(18),
          child: Container(
            margin: const EdgeInsets.only(bottom: 12),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            decoration: BoxDecoration(
              color: bgColor,
              borderRadius: BorderRadius.circular(18),
              border: Border.all(
                color: isOutgoing
                    ? const Color(0xFF154A3F)
                    : const Color(0xFFE2D6C3),
              ),
              boxShadow: const [
                BoxShadow(
                  blurRadius: 18,
                  color: Color(0x12000000),
                  offset: Offset(0, 8),
                ),
              ],
            ),
            child: Column(
              crossAxisAlignment: isSystem
                  ? CrossAxisAlignment.center
                  : CrossAxisAlignment.start,
              children: [
                if (isImage)
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (imageBytes != null)
                        ClipRRect(
                          borderRadius: BorderRadius.circular(14),
                          child: Image.memory(
                            imageBytes,
                            fit: BoxFit.cover,
                            gaplessPlayback: true,
                          ),
                        )
                      else
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 10,
                          ),
                          decoration: BoxDecoration(
                            color: isOutgoing
                                ? Colors.white.withValues(alpha: 0.12)
                                : const Color(0xFFF0E6D8),
                            borderRadius: BorderRadius.circular(12),
                          ),
                          child: Text(
                            'Image unavailable',
                            style: TextStyle(
                              color: isSystem
                                  ? const Color(0xFF5F4B37)
                                  : fgColor,
                            ),
                          ),
                        ),
                      if (message.content.trim().isNotEmpty) ...[
                        const SizedBox(height: 8),
                        Text(
                          message.content,
                          style: TextStyle(
                            color: isSystem ? const Color(0xFF5F4B37) : fgColor,
                            height: 1.35,
                          ),
                        ),
                      ],
                    ],
                  ),
                if (isAudio)
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        isPlaying
                            ? Icons.pause_circle_filled
                            : Icons.play_circle_fill,
                        color: isSystem ? const Color(0xFF5F4B37) : fgColor,
                        size: 22,
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          durationText.isEmpty
                              ? '${message.content}  Tap to play'
                              : '${message.content}  $durationText  Tap to play',
                          style: TextStyle(
                            color: isSystem ? const Color(0xFF5F4B37) : fgColor,
                            height: 1.35,
                          ),
                        ),
                      ),
                    ],
                  ),
                if (!isAudio && !isImage)
                  Text(
                    message.content,
                    style: TextStyle(
                      color: isSystem ? const Color(0xFF5F4B37) : fgColor,
                      height: 1.35,
                    ),
                  ),
                const SizedBox(height: 6),
                Text(
                  isImage
                      ? '${_formatTime(message.timestamp)}  Long press to copy'
                      : isAudio
                      ? '${_formatTime(message.timestamp)}  Tap to play · Long press to copy'
                      : '${_formatTime(message.timestamp)}  Long press to copy',
                  style: TextStyle(
                    fontSize: 11,
                    color: isOutgoing
                        ? Colors.white.withValues(alpha: 0.74)
                        : const Color(0xFF8C7863),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  static String _formatTime(DateTime time) {
    final hh = time.hour.toString().padLeft(2, '0');
    final mm = time.minute.toString().padLeft(2, '0');
    final ss = time.second.toString().padLeft(2, '0');
    return '$hh:$mm:$ss';
  }
}

enum _VoiceBubbleDirection { left, right }

class _VoiceActionBubble extends StatelessWidget {
  const _VoiceActionBubble({
    required this.width,
    required this.icon,
    required this.label,
    required this.helper,
    required this.active,
    required this.color,
    required this.direction,
  });

  final double width;
  final IconData icon;
  final String label;
  final String helper;
  final bool active;
  final Color color;
  final _VoiceBubbleDirection direction;

  @override
  Widget build(BuildContext context) {
    final bgColor = active ? color : Colors.white.withValues(alpha: 0.96);
    final fgColor = active ? Colors.white : color;
    return AnimatedContainer(
      duration: const Duration(milliseconds: 140),
      width: width,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(
          color: active ? color : const Color(0xFFD9C9AF),
          width: 1.4,
        ),
        boxShadow: [
          BoxShadow(
            blurRadius: active ? 18 : 10,
            color: active
                ? color.withValues(alpha: 0.34)
                : const Color(0x16000000),
            offset: const Offset(0, 6),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Icon(icon, size: 20, color: fgColor),
          const SizedBox(height: 6),
          Text(
            label,
            style: TextStyle(
              color: active ? Colors.white : const Color(0xFF5F4B37),
              fontWeight: FontWeight.w800,
              fontSize: 13,
              height: 1.15,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 3),
          Text(
            helper,
            style: TextStyle(
              color: active
                  ? Colors.white.withValues(alpha: 0.9)
                  : const Color(0xFF8A725B),
              fontWeight: FontWeight.w600,
              fontSize: 11,
              height: 1.1,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 6),
          Icon(
            direction == _VoiceBubbleDirection.left
                ? Icons.north_west_rounded
                : Icons.north_east_rounded,
            size: 18,
            color: fgColor.withValues(alpha: active ? 1 : 0.82),
          ),
        ],
      ),
    );
  }
}
