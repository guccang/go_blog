part of 'main.dart';

class AppAgentClient {
  static const Duration _httpTimeout = Duration(seconds: 8);
  static const Duration _uploadTimeout = Duration(minutes: 2);
  static const Duration _wsConnectTimeout = Duration(seconds: 8);

  AppAgentClient({
    required this.baseUrl,
    required this.userId,
    required this.password,
    required this.receiveToken,
    required this.sessionToken,
    this.obsAgentBaseUrl = '',
  });

  final String baseUrl;
  final String userId;
  final String password;
  final String receiveToken;
  final String sessionToken;
  final String obsAgentBaseUrl;

  Map<String, String> _sessionHeaders() {
    return <String, String>{
      if (receiveToken.trim().isNotEmpty)
        'X-App-Agent-Token': receiveToken.trim(),
      if (sessionToken.trim().isNotEmpty)
        'X-App-Agent-Session': sessionToken.trim(),
    };
  }

  Never _throwRequestError(String operation, http.Response resp) {
    final body = resp.body.trim();
    if (resp.statusCode == HttpStatus.unauthorized) {
      throw AppAgentUnauthorizedException(
        '$operation failed: ${resp.statusCode} $body',
      );
    }
    throw HttpException('$operation failed: ${resp.statusCode} $body');
  }

  Uri _buildAttachmentUri(String fileId) {
    final base = Uri.parse(baseUrl);
    final pathSegments = <String>[
      ...base.pathSegments.where((segment) => segment.isNotEmpty),
      'api',
      'app',
      'attachments',
      fileId,
    ];
    return base.replace(
      pathSegments: pathSegments,
      queryParameters: <String, String>{
        'user_id': userId,
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
      },
    );
  }

  Map<String, String> _attachmentHeaders({int? rangeStart}) {
    return <String, String>{
      if (receiveToken.trim().isNotEmpty)
        'X-App-Agent-Token': receiveToken.trim(),
      if (sessionToken.trim().isNotEmpty)
        'X-App-Agent-Session': sessionToken.trim(),
      if (rangeStart != null && rangeStart > 0)
        HttpHeaders.rangeHeader: 'bytes=$rangeStart-',
    };
  }

  Future<AppAuthSession> login() async {
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
      _throwRequestError('login', resp);
    }
    return AppAuthSession.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
      fallbackUserId: userId,
    );
  }

  Future<AppAuthSession> refreshSession(String refreshToken) async {
    final uri = Uri.parse('$baseUrl/api/app/refresh');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json',
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
          },
          body: jsonEncode({
            'user_id': userId,
            'refresh_token': refreshToken.trim(),
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('refresh', resp);
    }
    return AppAuthSession.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
      fallbackUserId: userId,
      fallbackRefreshToken: refreshToken,
    );
  }

  Future<void> logout({String refreshToken = ''}) async {
    final uri = Uri.parse('$baseUrl/api/app/logout');
    final resp = await http
        .post(
          uri,
          headers: <String, String>{
            HttpHeaders.contentTypeHeader: 'application/json',
            ..._sessionHeaders(),
          },
          body: jsonEncode({
            'user_id': userId,
            if (refreshToken.trim().isNotEmpty)
              'refresh_token': refreshToken.trim(),
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('logout', resp);
    }
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
            ..._sessionHeaders(),
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
      _throwRequestError('send', resp);
    }
  }

  Future<void> sendMessage(String content) => sendAppMessage(content);

  Future<AppResourceItem> uploadResourceBytes({
    required String category,
    required String fileName,
    required List<int> bytes,
    String contentType = '',
    String description = '',
  }) async {
    final uri = Uri.parse('$baseUrl/api/app/resources');
    final request = http.MultipartRequest('POST', uri)
      ..headers.addAll(_sessionHeaders())
      ..fields['user_id'] = userId
      ..fields['category'] = category
      ..fields['description'] = description;
    if (contentType.trim().isNotEmpty) {
      request.fields['content_type'] = contentType.trim();
    }
    request.files.add(
      http.MultipartFile.fromBytes('file', bytes, filename: fileName),
    );
    final streamed = await request.send().timeout(_uploadTimeout);
    final resp = await http.Response.fromStream(streamed);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('upload resource', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final item = data['item'];
    if (item is! Map<String, dynamic>) {
      throw const FormatException('upload resource response missing item');
    }
    return AppResourceItem.fromJson(item);
  }

  Future<AppResourceItem> uploadResourceFile({
    required String category,
    required String path,
    String description = '',
  }) async {
    final file = File(path);
    final fileName = path.split(Platform.pathSeparator).last.trim();
    if (!await file.exists()) {
      throw FileSystemException('file not found', path);
    }
    return uploadResourceBytes(
      category: category,
      fileName: fileName.isEmpty
          ? 'resource_${DateTime.now().millisecondsSinceEpoch}.bin'
          : fileName,
      bytes: await file.readAsBytes(),
      description: description,
    );
  }

  Future<AppResourceListResult> listResources(String category) async {
    final uri = Uri.parse('$baseUrl/api/app/resources').replace(
      queryParameters: <String, String>{
        'user_id': userId,
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
        if (category.trim().isNotEmpty) 'category': category.trim(),
      },
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list resources', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final usage = data['usage'] is Map<String, dynamic>
        ? AppResourceUsage.fromJson(data['usage'] as Map<String, dynamic>)
        : AppResourceUsage.fromJson(null);
    final items = (data['items'] as List<dynamic>? ?? const [])
        .map((item) => AppResourceItem.fromJson(item as Map<String, dynamic>))
        .toList();
    return AppResourceListResult(items: items, usage: usage);
  }

  Future<void> deleteResource(String fileId) async {
    final uri = Uri.parse('$baseUrl/api/app/resources').replace(
      queryParameters: <String, String>{
        'user_id': userId,
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
        'file_id': fileId,
      },
    );
    final resp = await http
        .delete(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('delete resource', resp);
    }
  }

  Future<void> sendCortanaEvent(
    String eventKind, {
    String content = '',
    Map<String, dynamic>? meta,
  }) {
    return sendAppMessage(
      content,
      messageType: 'event',
      meta: <String, dynamic>{
        'event_kind': eventKind,
        if (meta != null) ...meta,
      },
    );
  }

  Future<Map<String, dynamic>> fetchCortanaSettings() async {
    final uri = Uri.parse(
      '$baseUrl/api/app/cortana/settings?user_id=$userId&session_token=$sessionToken',
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('get cortana settings', resp);
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> saveCortanaSettings(
    Map<String, dynamic> settings,
  ) async {
    final uri = Uri.parse('$baseUrl/api/app/cortana/settings');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json',
            ..._sessionHeaders(),
          },
          body: jsonEncode(<String, dynamic>{'user_id': userId, ...settings}),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('save cortana settings', resp);
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  Future<List<GroupInfo>> listGroups() async {
    final uri = Uri.parse(
      '$baseUrl/api/app/groups?user_id=$userId&session_token=$sessionToken',
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list groups', resp);
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
            ..._sessionHeaders(),
          },
          body: jsonEncode({
            'action': action,
            'user_id': userId,
            'group_id': groupId,
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('group $action', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final groups = (data['groups'] as List<dynamic>? ?? const [])
        .map((item) => GroupInfo.fromJson(item as Map<String, dynamic>))
        .toList();
    return groups;
  }

  Future<CodegenProjectsSnapshot> listCodegenProjects() async {
    final uri = Uri.parse(
      '$baseUrl/api/app/codegen/projects?user_id=$userId&session_token=$sessionToken',
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list codegen projects', resp);
    }
    return CodegenProjectsSnapshot.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
    );
  }

  Future<Map<String, dynamic>> backupCodegenHistory({
    required CodegenHistoryBackupType backupType,
    required List<CodegenHistoryItem> history,
  }) async {
    final uri = Uri.parse('$baseUrl/api/app/codegen/history-backups');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json; charset=utf-8',
            ..._sessionHeaders(),
          },
          body: jsonEncode(<String, dynamic>{
            'user_id': userId,
            'backup_type': backupType.name,
            'app_version': appVersion,
            'platform': _platformLabel(),
            'history': history.map((item) => item.toJson()).toList(),
          }),
        )
        .timeout(_uploadTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('backup codegen history', resp);
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  Future<List<CodegenHistoryBackupItem>> listCodegenHistoryBackups() async {
    final uri = Uri.parse('$baseUrl/api/app/codegen/history-backups').replace(
      queryParameters: <String, String>{
        'user_id': userId,
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
      },
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list codegen history backups', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    return (data['items'] as List<dynamic>? ?? const [])
        .map(
          (item) =>
              CodegenHistoryBackupItem.fromJson(item as Map<String, dynamic>),
        )
        .where((item) => item.objectKey.isNotEmpty)
        .toList();
  }

  Future<List<CodegenHistoryItem>> loadCodegenHistoryBackup(
    String objectKey,
  ) async {
    final uri = Uri.parse('$baseUrl/api/app/codegen/history-backups').replace(
      queryParameters: <String, String>{
        'user_id': userId,
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
        'object_key': objectKey.trim(),
      },
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_uploadTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('load codegen history backup', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    return (data['history'] as List<dynamic>? ?? const [])
        .map(
          (item) => CodegenHistoryItem.fromJson(item as Map<String, dynamic>),
        )
        .where((item) => item.id.trim().isNotEmpty)
        .toList();
  }

  Future<List<CortanaReplayItem>> listCortanaVoiceHistory({
    int limit = 200,
  }) async {
    final uri = Uri.parse(
      '$baseUrl/api/app/cortana/history?user_id=$userId&session_token=$sessionToken&limit=$limit',
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list cortana history', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final items = (data['items'] as List<dynamic>? ?? const [])
        .map((item) {
          final json = item as Map<String, dynamic>;
          final createdAtValue = json['created_at'];
          final createdAtMillis = createdAtValue is int
              ? createdAtValue
              : int.tryParse('$createdAtValue') ??
                    DateTime.now().millisecondsSinceEpoch;
          return CortanaReplayItem(
            id: (json['id'] ?? '').toString().trim(),
            text: (json['text'] ?? '').toString().trim(),
            audioPath: '',
            audioFormat: (json['audio_format'] ?? '').toString().trim(),
            createdAt: DateTime.fromMillisecondsSinceEpoch(createdAtMillis),
            sourceLabel: '服务端历史',
            fileId: (json['file_id'] ?? '').toString().trim(),
            storageProvider: (json['storage_provider'] ?? '').toString().trim(),
            objectKey: (json['object_key'] ?? '').toString().trim(),
          );
        })
        .where((item) => item.text.isNotEmpty && item.fileId.isNotEmpty)
        .toList();
    return items;
  }

  Future<Map<String, dynamic>> createDebugBundle({
    required Map<String, dynamic> issue,
    required Map<String, dynamic> appState,
    required List<String> clientLogs,
    List<Map<String, dynamic>> timeline = const <Map<String, dynamic>>[],
  }) async {
    final uri = Uri.parse('$baseUrl/api/app/debug/bundles');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json',
            ..._sessionHeaders(),
          },
          body: jsonEncode(<String, dynamic>{
            'user_id': userId,
            'issue': issue,
            'app_state': appState,
            'timeline': timeline,
            'client_logs': clientLogs,
            'platform': _platformLabel(),
            'app_version': appVersion,
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('create debug bundle', resp);
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  Future<List<AppLogSource>> listLogSources() async {
    final uri = Uri.parse('$baseUrl/api/app/logs/sources').replace(
      queryParameters: <String, String>{
        'user_id': userId,
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
      },
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list log sources', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final sources = data['sources'];
    if (sources is! List) {
      return const <AppLogSource>[];
    }
    return sources
        .whereType<Map>()
        .map((item) => AppLogSource.fromJson(Map<String, dynamic>.from(item)))
        .where((item) => item.name.isNotEmpty)
        .toList(growable: false);
  }

  Future<AppLogContent> readLogContent({
    required String source,
    String file = '',
    int lines = 100,
  }) async {
    final query = <String, String>{
      'user_id': userId,
      'source': source,
      'lines': '${lines <= 0 ? 100 : lines}',
      if (file.trim().isNotEmpty) 'file': file.trim(),
      if (sessionToken.trim().isNotEmpty) 'session_token': sessionToken.trim(),
    };
    final uri = Uri.parse(
      '$baseUrl/api/app/logs/content',
    ).replace(queryParameters: query);
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('read log content', resp);
    }
    return AppLogContent.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
    );
  }

  Future<String> readDebugBundleFile({
    required String debugID,
    required String path,
  }) async {
    final uri = Uri.parse('$baseUrl/api/app/debug/bundles/$debugID/file')
        .replace(
          queryParameters: <String, String>{
            'user_id': userId,
            'path': path,
            if (sessionToken.trim().isNotEmpty)
              'session_token': sessionToken.trim(),
          },
        );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('read debug bundle file', resp);
    }
    return resp.body;
  }

  Future<WebSocketChannel> connectWebSocket() async {
    final uri = _buildWsUri(baseUrl, userId, sessionToken, receiveToken);
    final channel = WebSocketChannel.connect(uri);
    await channel.ready.timeout(_wsConnectTimeout);
    return channel;
  }

  Future<List<int>> downloadAttachment(String fileId) async {
    final uri = _buildAttachmentUri(fileId);
    final resp = await http.get(uri, headers: _attachmentHeaders());
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      if (resp.statusCode == HttpStatus.unauthorized) {
        throw AppAgentUnauthorizedException(
          'download attachment failed: ${resp.statusCode} ${resp.body}',
        );
      }
      throw HttpException(
        'download attachment failed: ${resp.statusCode} ${resp.body}',
      );
    }
    return resp.bodyBytes;
  }

  Future<void> downloadAttachmentToFile(
    String fileId, {
    required String destinationPath,
    Map<String, dynamic>? attachmentMeta,
    DownloadProgressCallback? onProgress,
  }) async {
    final downloader = const ResumableFileDownloader();
    final uri = _buildAttachmentUri(fileId);
    await downloader.downloadToFile(
      uri,
      destinationPath: destinationPath,
      headersBuilder: ({int? rangeStart}) =>
          _attachmentHeaders(rangeStart: rangeStart),
      onProgress: onProgress,
    );
  }

  static Uri _buildWsUri(
    String baseUrl,
    String userId,
    String sessionToken,
    String receiveToken,
  ) {
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
        if (receiveToken.trim().isNotEmpty) 'token': receiveToken.trim(),
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
      },
    );
  }
}

class AppLogSource {
  const AppLogSource({
    required this.name,
    this.path = '',
    this.description = '',
  });

  final String name;
  final String path;
  final String description;

  factory AppLogSource.fromJson(Map<String, dynamic> json) {
    return AppLogSource(
      name: (json['name'] ?? '').toString().trim(),
      path: (json['path'] ?? '').toString().trim(),
      description: (json['description'] ?? '').toString().trim(),
    );
  }
}

class AppLogContent {
  const AppLogContent({
    required this.source,
    required this.file,
    required this.content,
    this.logPath = '',
    this.matchedLines = 0,
    this.truncated = false,
  });

  final AppLogSource source;
  final String file;
  final String logPath;
  final String content;
  final int matchedLines;
  final bool truncated;

  factory AppLogContent.fromJson(Map<String, dynamic> json) {
    final source = json['source'];
    return AppLogContent(
      source: source is Map
          ? AppLogSource.fromJson(Map<String, dynamic>.from(source))
          : const AppLogSource(name: ''),
      file: (json['file'] ?? '').toString().trim(),
      logPath: (json['log_path'] ?? '').toString().trim(),
      content: (json['content'] ?? '').toString(),
      matchedLines: json['matched_lines'] is int
          ? json['matched_lines'] as int
          : int.tryParse('${json['matched_lines'] ?? 0}') ?? 0,
      truncated: json['truncated'] == true,
    );
  }
}
