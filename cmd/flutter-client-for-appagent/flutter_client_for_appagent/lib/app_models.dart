part of 'main.dart';

class LlmDebugEvent {
  const LlmDebugEvent({
    required this.event,
    required this.content,
    required this.timestamp,
    required this.meta,
  });

  final String event;
  final String content;
  final DateTime timestamp;
  final Map<String, dynamic> meta;

  String get label {
    switch (event) {
      case 'debug_prompt':
        return '提示词';
      case 'debug_llm_round':
        return 'LLM';
      case 'tool_call':
        return '工具调用';
      case 'tool_result':
        return '工具结果';
      case 'thinking':
        return '思考';
      case 'tool_info':
        return '工具';
      case 'task_complete':
        return '完成';
      default:
        return event;
    }
  }

  Map<String, dynamic> get payload {
    final raw = meta['debug_payload'];
    if (raw is Map<String, dynamic>) {
      return raw;
    }
    if (raw is Map) {
      return Map<String, dynamic>.from(raw);
    }
    return const <String, dynamic>{};
  }

  String get detailText {
    final data = payload;
    if (event == 'debug_prompt') {
      final prompt = (data['system_prompt'] ?? '').toString();
      if (prompt.trim().isNotEmpty) {
        return prompt;
      }
    }
    if (event == 'debug_llm_round') {
      final assistant = (data['assistant_text'] ?? '').toString();
      final calls = data['tool_calls'];
      final parts = <String>[];
      if (assistant.trim().isNotEmpty) {
        parts.add(assistant);
      }
      if (calls is List && calls.isNotEmpty) {
        parts.add(const JsonEncoder.withIndent('  ').convert(calls));
      }
      if (parts.isNotEmpty) {
        return parts.join('\n\n');
      }
    }
    return content;
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
    sanitizedMeta?.remove('image_base64');
    sanitizedMeta?.remove('video_base64');
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

bool isApkChatMessage(ChatMessage message) {
  if (message.messageType != 'file') {
    return false;
  }
  final fileName = (message.meta?['file_name'] ?? '')
      .toString()
      .trim()
      .toLowerCase();
  final fileFormat = (message.meta?['file_format'] ?? '')
      .toString()
      .trim()
      .toLowerCase();
  return fileName.endsWith('.apk') || fileFormat == 'apk';
}

/// Extract version string from APK filename.
/// Examples: "app-release-1.0.0.apk" -> "1.0.0", "myapp-2.3.4+5.apk" -> "2.3.4+5"
String? extractApkVersion(ChatMessage message) {
  final fileName = (message.meta?['file_name'] ?? '').toString().trim();
  final match = RegExp(
    r'[-_](\d+\.\d+\.\d+(?:\+\d+)?)[^.]*\.apk$',
    caseSensitive: false,
  ).firstMatch(fileName);
  return match?.group(1);
}

/// Extract version string from APK filename.
/// Examples: "app-release-1.0.0.apk" -> "1.0.0", "myapp-2.3.4+5.apk" -> "2.3.4+5"
String? extractApkVersionFromString(String fileName) {
  final match = RegExp(
    r'[-_](\d+\.\d+\.\d+(?:\+\d+)?)[^.]*\.apk$',
    caseSensitive: false,
  ).firstMatch(fileName);
  return match?.group(1);
}

/// Compare two version strings.
/// Returns 1 if versionA > versionB, 0 if equal, -1 if versionA < versionB.
int compareApkVersions(String? versionA, String? versionB) {
  if (versionA == null && versionB == null) return 0;
  if (versionA == null) return -1;
  if (versionB == null) return 1;

  // Parse version parts (e.g., "1.2.3" -> [1, 2, 3])
  List<int> parseParts(String v) {
    // Remove build metadata part after +
    final baseV = v.split('+')[0];
    final parts = baseV.split('.');
    final result = <int>[];
    for (final part in parts) {
      final num = int.tryParse(part);
      result.add(num ?? 0);
    }
    return result;
  }

  final partsA = parseParts(versionA);
  final partsB = parseParts(versionB);

  // Compare each part
  final maxLen = partsA.length > partsB.length ? partsA.length : partsB.length;
  for (int i = 0; i < maxLen; i++) {
    final valA = i < partsA.length ? partsA[i] : 0;
    final valB = i < partsB.length ? partsB[i] : 0;

    if (valA > valB) return 1;
    if (valA < valB) return -1;
  }

  return 0;
}

enum MessageDirection { outgoing, incoming, system }

enum _AttachmentMenuAction {
  galleryImage,
  cameraImage,
  imageResource,
  live2dResource,
  fileResource,
  browseResources,
}

enum VoiceGestureAction { sendAudio, cancel, transcribe }

const double _voiceGestureVerticalThreshold = 48;
const double _voiceGestureHorizontalThreshold = 24;
const double _voiceGestureStrongHorizontalThreshold = 72;
const double _voiceGestureHorizontalGraceDy = 24;

VoiceGestureAction resolveVoiceGestureAction(Offset dragOffset) {
  final dx = dragOffset.dx;
  final dy = dragOffset.dy;
  final movedUp = dy <= -_voiceGestureVerticalThreshold;
  final movedLeft = dx <= -_voiceGestureHorizontalThreshold;
  final movedRight = dx >= _voiceGestureHorizontalThreshold;
  final strongLeftSwipe =
      dx <= -_voiceGestureStrongHorizontalThreshold &&
      dy <= _voiceGestureHorizontalGraceDy;
  final strongRightSwipe =
      dx >= _voiceGestureStrongHorizontalThreshold &&
      dy <= _voiceGestureHorizontalGraceDy;

  if ((movedUp && movedLeft) || strongLeftSwipe) {
    return VoiceGestureAction.cancel;
  }
  if ((movedUp && movedRight) || strongRightSwipe) {
    return VoiceGestureAction.transcribe;
  }
  return VoiceGestureAction.sendAudio;
}

class PushEnvelope {
  PushEnvelope({
    required this.messageId,
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
      messageId: (json['message_id'] ?? '').toString(),
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

  final String messageId;
  final int sequence;
  final String userId;
  final String content;
  final String channel;
  final String messageType;
  final int timestamp;
  final Map<String, dynamic>? meta;
}

typedef ScopedHistoryPersistCallback = Future<void> Function(String scopeKey);

String resolvePreferredGroupId(
  List<GroupInfo> groups, {
  String? preferredGroupId,
  bool allowImplicitSingleSelection = true,
}) {
  if (groups.isEmpty) {
    return '';
  }
  final preferred = preferredGroupId?.trim() ?? '';
  if (preferred.isNotEmpty && groups.any((group) => group.id == preferred)) {
    return preferred;
  }
  if (allowImplicitSingleSelection && groups.length == 1) {
    return groups.first.id;
  }
  return '';
}

class ScopedHistoryPersistenceCoordinator {
  ScopedHistoryPersistenceCoordinator(this._persist);

  final ScopedHistoryPersistCallback _persist;
  final Map<String, Future<void>> _tails = <String, Future<void>>{};
  final Map<String, int> _revisions = <String, int>{};
  int _epoch = 0;

  void schedule(String scopeKey) {
    final normalizedScopeKey = scopeKey.trim();
    if (normalizedScopeKey.isEmpty) {
      return;
    }
    final scheduledEpoch = _epoch;
    final revision = (_revisions[normalizedScopeKey] ?? 0) + 1;
    _revisions[normalizedScopeKey] = revision;

    final previous = _tails[normalizedScopeKey] ?? Future<void>.value();
    late final Future<void> future;
    future = previous.catchError((_) {}).then((_) async {
      // 同一 scope 的并发写只落最后一次，避免旧快照覆盖新消息。
      if (_epoch != scheduledEpoch) {
        return;
      }
      if (_revisions[normalizedScopeKey] != revision) {
        return;
      }
      await _persist(normalizedScopeKey);
    });
    _tails[normalizedScopeKey] = future.whenComplete(() {
      if (identical(_tails[normalizedScopeKey], future)) {
        _tails.remove(normalizedScopeKey);
      }
    });
  }

  void invalidate() {
    _epoch++;
    _revisions.clear();
  }

  Future<void> flushScope(String scopeKey) async {
    final normalizedScopeKey = scopeKey.trim();
    if (normalizedScopeKey.isEmpty) {
      return;
    }
    schedule(normalizedScopeKey);
    final tail = _tails[normalizedScopeKey];
    if (tail == null) {
      return;
    }
    await tail.catchError((_) {});
  }

  Future<void> flushAll([Iterable<String>? scopeKeys]) async {
    final normalizedScopeKeys =
        (scopeKeys ?? <String>[..._revisions.keys, ..._tails.keys])
            .map((scopeKey) => scopeKey.trim())
            .where((scopeKey) => scopeKey.isNotEmpty)
            .toSet()
            .toList();
    if (normalizedScopeKeys.isEmpty) {
      return;
    }
    for (final scopeKey in normalizedScopeKeys) {
      schedule(scopeKey);
    }
    await Future.wait(normalizedScopeKeys.map(flushScope), eagerError: false);
  }
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

enum RootTab { chat, codegen, cortana, debug, settings }

class ClientConfig {
  const ClientConfig({
    required this.baseUrl,
    required this.receiveToken,
    required this.enableLocalVosk,
    required this.voskModelPath,
    required this.cortanaEnabledDefault,
    required this.cortanaAllowFullAccessDefault,
    required this.cortanaAutoPlayDefault,
    required this.cortanaProactiveModeDefault,
    required this.cortanaHighFreqStartHourDefault,
    required this.cortanaHighFreqStartMinuteDefault,
    required this.cortanaHighFreqEndHourDefault,
    required this.cortanaHighFreqEndMinuteDefault,
    required this.cortanaPersonaNameDefault,
    required this.cortanaPersonaDescriptionDefault,
  });

  final String baseUrl;
  final String receiveToken;
  final bool enableLocalVosk;
  final String voskModelPath;
  final bool cortanaEnabledDefault;
  final bool cortanaAllowFullAccessDefault;
  final bool cortanaAutoPlayDefault;
  final String cortanaProactiveModeDefault;
  final int cortanaHighFreqStartHourDefault;
  final int cortanaHighFreqStartMinuteDefault;
  final int cortanaHighFreqEndHourDefault;
  final int cortanaHighFreqEndMinuteDefault;
  final String cortanaPersonaNameDefault;
  final String cortanaPersonaDescriptionDefault;

  factory ClientConfig.fromJson(Map<String, dynamic> json) {
    return ClientConfig(
      baseUrl: (json['base_url'] ?? '').toString().trim(),
      receiveToken: (json['receive_token'] ?? '').toString().trim(),
      enableLocalVosk: json['enable_local_vosk'] == true,
      voskModelPath: (json['vosk_model_path'] ?? '').toString().trim(),
      cortanaEnabledDefault: json['cortana_enabled_default'] != false,
      cortanaAllowFullAccessDefault:
          json['cortana_allow_full_access_default'] != false,
      cortanaAutoPlayDefault: json['cortana_auto_play_default'] != false,
      cortanaProactiveModeDefault:
          (json['cortana_proactive_mode_default'] ?? 'high').toString().trim(),
      cortanaHighFreqStartHourDefault:
          (json['cortana_high_freq_start_hour_default'] as num?)?.toInt() ?? 9,
      cortanaHighFreqStartMinuteDefault:
          (json['cortana_high_freq_start_minute_default'] as num?)?.toInt() ??
          0,
      cortanaHighFreqEndHourDefault:
          (json['cortana_high_freq_end_hour_default'] as num?)?.toInt() ?? 22,
      cortanaHighFreqEndMinuteDefault:
          (json['cortana_high_freq_end_minute_default'] as num?)?.toInt() ?? 0,
      cortanaPersonaNameDefault:
          (json['cortana_persona_name_default'] ?? 'Cortana').toString().trim(),
      cortanaPersonaDescriptionDefault:
          (json['cortana_persona_description_default'] ?? '').toString().trim(),
    );
  }
}

class AppAgentUnauthorizedException implements Exception {
  const AppAgentUnauthorizedException(this.message);

  final String message;

  @override
  String toString() => message;
}

class AppAuthSession {
  const AppAuthSession({
    required this.userId,
    required this.accessToken,
    required this.refreshToken,
    required this.expiresAtMs,
    required this.obsAgentBaseUrl,
  });

  final String userId;
  final String accessToken;
  final String refreshToken;
  final int expiresAtMs;
  final String obsAgentBaseUrl;

  factory AppAuthSession.fromJson(
    Map<String, dynamic> json, {
    String fallbackUserId = '',
    String fallbackRefreshToken = '',
    String fallbackObsAgentBaseUrl = '',
  }) {
    final accessToken = (json['access_token'] ?? json['session_token'] ?? '')
        .toString()
        .trim();
    final refreshToken = (json['refresh_token'] ?? fallbackRefreshToken)
        .toString()
        .trim();
    final userId = (json['user_id'] ?? fallbackUserId).toString().trim();
    final obsAgentBaseUrl =
        (json['obs_agent_base_url'] ?? fallbackObsAgentBaseUrl)
            .toString()
            .trim();
    final expiresAtMs = _resolveExpiresAtMs(json);
    if (accessToken.isEmpty) {
      throw const FormatException('missing access_token');
    }
    if (userId.isEmpty) {
      throw const FormatException('missing user_id');
    }
    return AppAuthSession(
      userId: userId,
      accessToken: accessToken,
      refreshToken: refreshToken,
      expiresAtMs: expiresAtMs,
      obsAgentBaseUrl: obsAgentBaseUrl,
    );
  }

  static int _resolveExpiresAtMs(Map<String, dynamic> json) {
    final rawExpiresAt = json['expires_at'];
    if (rawExpiresAt is int) {
      return rawExpiresAt;
    }
    final parsedExpiresAt = int.tryParse('${json['expires_at']}') ?? 0;
    if (parsedExpiresAt > 0) {
      return parsedExpiresAt;
    }
    final rawExpiresIn = json['expires_in'];
    final expiresIn = rawExpiresIn is int
        ? rawExpiresIn
        : int.tryParse('${json['expires_in']}') ?? 0;
    if (expiresIn <= 0) {
      return 0;
    }
    return DateTime.now().millisecondsSinceEpoch + expiresIn * 1000;
  }
}
