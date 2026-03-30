import 'dart:async';
import 'dart:convert' show base64Encode, jsonDecode, jsonEncode, utf8;
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
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
          primary: Color(0xFF0E3B2E),
          secondary: Color(0xFFC47B2A),
          surface: Color(0xFFF7F2E8),
        ),
        scaffoldBackgroundColor: const Color(0xFFF7F2E8),
        inputDecorationTheme: const InputDecorationTheme(
          border: OutlineInputBorder(),
          filled: true,
          fillColor: Colors.white,
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
  final _userIdController = TextEditingController(text: 'demo-user');
  final _passwordController = TextEditingController();
  final _groupIdController = TextEditingController();
  final _messageController = TextEditingController();
  final _scrollController = ScrollController();
  final AudioRecorder _audioRecorder = AudioRecorder();
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
  bool _autoReconnect = false;
  bool _configLoading = true;
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
    _groupIdController.dispose();
    _messageController.dispose();
    _scrollController.dispose();
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
      final raw = await rootBundle.loadString('assets/app_config.json');
      final config = ClientConfig.fromJson(
        jsonDecode(raw) as Map<String, dynamic>,
      );
      if (config.baseUrl.isEmpty) {
        throw const FormatException('base_url is required');
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _clientConfig = config;
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

  void _appendIncoming(String text, DateTime when) {
    _appendMessage(
      ChatMessage(
        content: text,
        direction: MessageDirection.incoming,
        timestamp: when,
        scopeKey: _currentScopeKey,
      ),
      updateStatus: 'Received message',
    );
  }

  void _appendOutgoing(String text) {
    _appendMessage(
      ChatMessage(
        content: text,
        direction: MessageDirection.outgoing,
        timestamp: DateTime.now(),
        scopeKey: _currentScopeKey,
        authorId: _userIdController.text.trim(),
        groupId: _currentGroupId,
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
      _appendOutgoing(label);
      setState(() {
        _sending = true;
      });
      try {
        await _client.sendAppMessage(
          label,
          messageType: 'audio',
          meta: <String, dynamic>{
            'audio_base64': base64Encode(bytes),
            'audio_format': Platform.isWindows ? 'wav' : 'm4a',
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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('App Agent Client'),
        backgroundColor: const Color(0xFFF0E8D8),
      ),
      body: Column(
        children: [
          Container(
            color: const Color(0xFFF0E8D8),
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 10),
            child: Column(
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        _configLoading
                            ? 'Loading client config...'
                            : (_clientConfig == null
                                  ? (_configError.isEmpty
                                        ? 'Client config unavailable'
                                        : _configError)
                                  : 'Client config loaded'),
                        style: TextStyle(
                          color: _clientConfig == null
                              ? Colors.brown[500]
                              : Colors.green[700],
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _userIdController,
                        decoration: const InputDecoration(
                          labelText: 'User ID',
                          hintText: 'demo-user',
                        ),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: TextField(
                        controller: _passwordController,
                        obscureText: true,
                        decoration: const InputDecoration(
                          labelText: 'Password',
                          hintText: 'blog-agent password',
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                Row(
                  children: [
                    FilledButton.icon(
                      onPressed:
                          _loggingIn || _configLoading || _clientConfig == null
                          ? null
                          : _login,
                      icon: const Icon(Icons.login),
                      label: Text(_sessionToken.isEmpty ? 'Login' : 'Re-login'),
                    ),
                    const SizedBox(width: 8),
                    OutlinedButton.icon(
                      onPressed: _connected || _connecting
                          ? _disconnectWs
                          : null,
                      icon: const Icon(Icons.link_off),
                      label: const Text('Disconnect'),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        _connected
                            ? 'WebSocket online'
                            : (_connecting
                                  ? 'Connecting...'
                                  : (_sessionToken.isEmpty
                                        ? 'Login required'
                                        : 'WebSocket offline')),
                        textAlign: TextAlign.right,
                        style: TextStyle(
                          color: _connected
                              ? Colors.green[700]
                              : Colors.brown[400],
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    _clientConfig == null
                        ? 'WS endpoint unavailable until client config loads.'
                        : 'WS endpoint ready.',
                    style: TextStyle(color: Colors.brown[700], fontSize: 12),
                  ),
                ),
                const SizedBox(height: 4),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    _sessionToken.isEmpty
                        ? 'Login uses blog-agent account verification. User ID maps to account.'
                        : 'Login ok. Offline messages are cached in app-agent and flushed after reconnect.',
                    style: TextStyle(color: Colors.brown[500], fontSize: 12),
                  ),
                ),
                const SizedBox(height: 10),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _groupIdController,
                        decoration: const InputDecoration(
                          labelText: 'Group ID',
                          hintText: 'party-01',
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    FilledButton.tonal(
                      onPressed: _sessionToken.isEmpty
                          ? null
                          : () => _mutateGroup('create'),
                      child: const Text('Create'),
                    ),
                    const SizedBox(width: 8),
                    FilledButton.tonal(
                      onPressed: _sessionToken.isEmpty
                          ? null
                          : () => _mutateGroup('join'),
                      child: const Text('Join'),
                    ),
                    const SizedBox(width: 8),
                    OutlinedButton(
                      onPressed: _sessionToken.isEmpty
                          ? null
                          : () => _mutateGroup('leave'),
                      child: const Text('Leave'),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      ChoiceChip(
                        selected: _currentGroupId.isEmpty,
                        label: const Text('Direct'),
                        onSelected: (_) => unawaited(_switchToDirectScope()),
                      ),
                      ..._groups.map(
                        (group) => ChoiceChip(
                          selected: _currentGroupId == group.id,
                          label: Text('${group.id} (${group.members.length})'),
                          onSelected: (_) =>
                              unawaited(_switchToGroupScope(group.id)),
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    _currentGroupId.isEmpty
                        ? 'Current scope: direct chat'
                        : 'Current scope: group ${_currentGroupId.toLowerCase()}',
                    style: TextStyle(
                      color: Colors.brown[700],
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                if (_recording) ...[
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: _VoiceActionBadge(
                          icon: Icons.close,
                          label: '左上滑取消',
                          active: _cancelVoiceAction,
                          color: const Color(0xFF9B2C2C),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: _VoiceActionBadge(
                          icon: Icons.subtitles,
                          label: '右上滑转文字',
                          active: _speechVoiceAction,
                          color: const Color(0xFF0E5A44),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Align(
                    alignment: Alignment.centerLeft,
                    child: Text(
                      _speechDraft.isEmpty
                          ? '正在录音，松手发送语音'
                          : '识别中: $_speechDraft',
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: Colors.brown[700],
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
          Expanded(
            child: ListView.builder(
              controller: _scrollController,
              padding: const EdgeInsets.all(16),
              itemCount: _messages.length,
              itemBuilder: (context, index) {
                final msg = _messages[index];
                return _MessageBubble(
                  message: msg,
                  onCopy: () async {
                    await Clipboard.setData(ClipboardData(text: msg.content));
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
          Container(
            decoration: const BoxDecoration(
              color: Colors.white,
              boxShadow: [
                BoxShadow(
                  blurRadius: 18,
                  color: Color(0x12000000),
                  offset: Offset(0, -6),
                ),
              ],
            ),
            padding: const EdgeInsets.fromLTRB(16, 14, 16, 18),
            child: Column(
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        _status,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: Colors.brown[700]),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    GestureDetector(
                      onLongPressStart: _handleVoiceStart,
                      onLongPressMoveUpdate: _handleVoiceMove,
                      onLongPressEnd: _handleVoiceEnd,
                      child: Container(
                        height: 52,
                        width: 52,
                        decoration: BoxDecoration(
                          color: _recording
                              ? const Color(0xFF9B2C2C)
                              : const Color(0xFFE8DCC7),
                          borderRadius: BorderRadius.circular(16),
                          border: Border.all(color: const Color(0xFFD9C9AF)),
                        ),
                        child: Icon(
                          _recording ? Icons.mic : Icons.mic_none,
                          color: _recording
                              ? Colors.white
                              : const Color(0xFF5F4B37),
                        ),
                      ),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: TextField(
                        controller: _messageController,
                        minLines: 1,
                        maxLines: 5,
                        enabled: !_recording,
                        onSubmitted: (_) => _sendMessage(),
                        decoration: const InputDecoration(
                          labelText: 'Message',
                          hintText: 'Ask something... or hold mic',
                        ),
                      ),
                    ),
                    const SizedBox(width: 10),
                    FilledButton.icon(
                      onPressed: (_sending || _recording) ? null : _sendMessage,
                      icon: const Icon(Icons.send),
                      label: const Text('Send'),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _MessageBubble extends StatelessWidget {
  const _MessageBubble({required this.message, required this.onCopy});

  final ChatMessage message;
  final Future<void> Function() onCopy;

  @override
  Widget build(BuildContext context) {
    final isOutgoing = message.direction == MessageDirection.outgoing;
    final isSystem = message.direction == MessageDirection.system;
    final alignment = isSystem
        ? Alignment.center
        : (isOutgoing ? Alignment.centerRight : Alignment.centerLeft);
    final bgColor = isSystem
        ? const Color(0xFFE8DCC7)
        : (isOutgoing ? const Color(0xFF0E3B2E) : const Color(0xFFFFFFFF));
    final fgColor = isOutgoing ? Colors.white : const Color(0xFF2D241F);

    return Align(
      alignment: alignment,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 720),
        child: InkWell(
          onLongPress: () => onCopy(),
          borderRadius: BorderRadius.circular(18),
          child: Container(
            margin: const EdgeInsets.only(bottom: 12),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            decoration: BoxDecoration(
              color: bgColor,
              borderRadius: BorderRadius.circular(18),
              border: Border.all(color: const Color(0xFFD9C9AF)),
            ),
            child: Column(
              crossAxisAlignment: isSystem
                  ? CrossAxisAlignment.center
                  : CrossAxisAlignment.start,
              children: [
                Text(
                  message.content,
                  style: TextStyle(
                    color: isSystem ? const Color(0xFF5F4B37) : fgColor,
                    height: 1.35,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  '${_formatTime(message.timestamp)}  Long press to copy',
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

class _VoiceActionBadge extends StatelessWidget {
  const _VoiceActionBadge({
    required this.icon,
    required this.label,
    required this.active,
    required this.color,
  });

  final IconData icon;
  final String label;
  final bool active;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return AnimatedContainer(
      duration: const Duration(milliseconds: 120),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: active ? color : Colors.white,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: active ? color : const Color(0xFFD9C9AF)),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(icon, size: 18, color: active ? Colors.white : color),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              color: active ? Colors.white : const Color(0xFF5F4B37),
              fontWeight: FontWeight.w600,
              fontSize: 12,
            ),
          ),
        ],
      ),
    );
  }
}
