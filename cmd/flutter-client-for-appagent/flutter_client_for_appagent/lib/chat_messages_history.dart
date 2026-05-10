// ignore_for_file: invalid_use_of_protected_member
part of 'main.dart';

extension _ChatPageStateMessagesHistory on _ChatPageState {
  String get _currentScopeKey =>
      _currentGroupId.isEmpty ? 'direct' : _groupScopeKey(_currentGroupId);

  List<ChatMessage> get _messages =>
      _historyByScope[_currentScopeKey] ?? const <ChatMessage>[];

  String _resolvePreferredGroupId(
    List<GroupInfo> groups, {
    String? preferredGroupId,
    bool allowImplicitSingleSelection = true,
  }) {
    return resolvePreferredGroupId(
      groups,
      preferredGroupId: preferredGroupId,
      allowImplicitSingleSelection: allowImplicitSingleSelection,
    );
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

  List<String> _recentFlutterClientLogLines({int limit = 200}) {
    final entries = List<FlutterClientLogEntry>.from(
      flutterClientLogs,
    ).take(limit).toList(growable: false).reversed;
    return entries
        .map(
          (entry) =>
              '${entry.timestamp.toIso8601String()} ${entry.message.trim()}',
        )
        .where((line) => line.trim().isNotEmpty)
        .toList(growable: false);
  }

  Future<void> _copyRecentFlutterClientLogs() async {
    final text = _recentFlutterClientLogLines().join('\n');
    if (text.trim().isEmpty) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('暂无 Flutter 客户端日志'),
          duration: Duration(seconds: 1),
        ),
      );
      return;
    }
    await _copyText('Flutter client logs', text);
  }

  Future<void> _reportDebugAppState() async {
    if (_clientConfig == null || _sessionToken.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('请先登录后再上报调试状态'),
          duration: Duration(seconds: 1),
        ),
      );
      return;
    }
    final timeline = _llmDebugEvents.reversed
        .take(80)
        .map(
          (event) => <String, dynamic>{
            'time': event.timestamp.toIso8601String(),
            'category': 'llm',
            'event': event.event,
            'label': event.label,
            'message': event.content,
            'payload': event.payload,
          },
        )
        .toList(growable: false);
    final appState = <String, dynamic>{
      'root_tab': _rootTab.name,
      'connection': _connectionLabel,
      'websocket_connected': _connected,
      'current_group_id': _currentGroupId,
      'message_count': _messages.length,
      'llm_debug_event_count': _llmDebugEvents.length,
      'cortana_mode': _cortanaFloatingMode.name,
      'cortana_enabled': _cortanaEnabled,
      'base_url': _clientConfig?.baseUrl ?? '',
    };
    try {
      final resp = await _client.createDebugBundle(
        issue: <String, dynamic>{
          'title': 'Flutter App 客户端状态上报',
          'user_description': '用户从 App 调试页上报当前页面状态和最近客户端日志。',
          'repro_steps': <String>['打开 App 调试页', '点击上报当前状态'],
        },
        appState: appState,
        timeline: timeline,
        clientLogs: _recentFlutterClientLogLines(),
      );
      final debugId = (resp['debug_id'] ?? '').toString();
      final bundlePath = (resp['bundle_path'] ?? '').toString();
      addFlutterClientLog('Debug Bundle 已创建: $debugId $bundlePath');
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            debugId.isEmpty ? 'Debug Bundle 已创建' : 'Debug Bundle: $debugId',
          ),
          duration: const Duration(seconds: 3),
        ),
      );
    } catch (err) {
      addFlutterClientLog('Debug Bundle 创建失败: $err');
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_describeRequestError(err, operation: 'Debug Bundle')),
          duration: const Duration(seconds: 3),
        ),
      );
    }
  }

  Future<Map<String, String>> _createCodegenDebugBundle() async {
    if (_clientConfig == null || _sessionToken.trim().isEmpty) {
      _appendSystem('请先登录后再创建 Debug Bundle。');
      return const <String, String>{};
    }
    final prompt = _codegenPromptController.text.trim();
    final timeline = _llmDebugEvents.reversed
        .take(80)
        .map(
          (event) => <String, dynamic>{
            'time': event.timestamp.toIso8601String(),
            'category': 'llm',
            'event': event.event,
            'label': event.label,
            'message': event.content,
            'payload': event.payload,
          },
        )
        .toList(growable: false);
    final appState = <String, dynamic>{
      'root_tab': _rootTab.name,
      'connection': _connectionLabel,
      'websocket_connected': _connected,
      'current_group_id': _currentGroupId,
      'message_count': _messages.length,
      'llm_debug_event_count': _llmDebugEvents.length,
      'cortana_mode': _cortanaFloatingMode.name,
      'cortana_enabled': _cortanaEnabled,
      'base_url': _clientConfig?.baseUrl ?? '',
      'codegen_project': _selectedCodingProject?.qualifiedName ?? '',
      'codegen_tool': _selectedCodeTool,
      'codegen_settings': _selectedClaudeSettings,
    };
    try {
      final resp = await _runAuthed('Create codegen debug bundle', (client) {
        return client.createDebugBundle(
          issue: <String, dynamic>{
            'title': 'Flutter App 编码调试',
            'user_description': prompt.isEmpty
                ? '用户从编码发布页发起 ACP debug 会话。'
                : prompt,
            'repro_steps': <String>['打开编码发布页', '启用携带 Debug Bundle', '发送编码调试命令'],
          },
          appState: appState,
          timeline: timeline,
          clientLogs: _recentFlutterClientLogLines(),
        );
      });
      final debugId = (resp['debug_id'] ?? '').toString().trim();
      final bundlePath = (resp['bundle_path'] ?? '').toString().trim();
      addFlutterClientLog('Codegen Debug Bundle 已创建: $debugId $bundlePath');
      if (debugId.isEmpty) {
        _appendSystem('Debug Bundle 创建成功，但服务端没有返回 debug_id。');
      } else {
        _appendSystem('Debug Bundle 已创建: $debugId');
      }
      return <String, String>{'debug_id': debugId, 'debug_path': bundlePath};
    } catch (err) {
      addFlutterClientLog('Codegen Debug Bundle 创建失败: $err');
      _appendSystem(_describeRequestError(err, operation: 'Debug Bundle'));
      return const <String, String>{};
    }
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

  void _showGreetingSnackBar(String text) {
    if (!mounted) {
      return;
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      final messenger = ScaffoldMessenger.maybeOf(context);
      if (messenger == null) {
        return;
      }
      messenger
        ..hideCurrentSnackBar()
        ..showSnackBar(
          SnackBar(content: Text(text), duration: const Duration(seconds: 2)),
        );
    });
  }

  void _maybeShowStartupGreeting() {
    if (_startupGreetingShown) {
      return;
    }
    _startupGreetingShown = true;
    _appendSystem('欢迎来到 App Agent，可以先登录，也可以先查看当前配置。');
    _showGreetingSnackBar('欢迎来到 App Agent');
  }

  void _maybeShowLoginGreeting({required bool restored}) {
    if (_loginGreetingShown) {
      return;
    }
    _loginGreetingShown = true;
    final text = restored ? '欢迎回来，连接已恢复，可以继续对话。' : '登录成功，欢迎回来，可以开始对话。';
    _appendSystem(text);
    _showGreetingSnackBar(restored ? '欢迎回来' : '登录成功');
  }

  void _appendOutgoing(
    String text, {
    String messageType = 'text',
    Map<String, dynamic>? meta,
    String? scopeKeyOverride,
    String? groupIdOverride,
  }) {
    final scopeKey = scopeKeyOverride ?? _currentScopeKey;
    final groupId = groupIdOverride ?? _currentGroupId;
    _appendMessage(
      ChatMessage(
        content: text,
        direction: MessageDirection.outgoing,
        timestamp: DateTime.now(),
        scopeKey: scopeKey,
        authorId: _userIdController.text.trim(),
        groupId: groupId,
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
        _historyPersistence.schedule(message.scopeKey);
      }
      return;
    }
    setState(() {
      if (updateStatus != null) {
        _status = updateStatus;
      }
    });
    if (persist) {
      _historyPersistence.schedule(message.scopeKey);
    }
    if (message.scopeKey == _currentScopeKey) {
      _scrollToBottom();
      if (message.direction == MessageDirection.incoming) {
        unawaited(_markScopeAsRead(message.scopeKey));
        _triggerCortanaContextualExpression('surprised');
      }
    }
  }

  bool _replaceMessageById({
    required String scopeKey,
    required String messageId,
    required ChatMessage message,
    String? updateStatus,
  }) {
    if (messageId.trim().isEmpty) {
      return false;
    }
    final history = _historyByScope[scopeKey];
    if (history == null || history.isEmpty) {
      return false;
    }
    final index = history.lastIndexWhere(
      (item) => (item.meta?['_message_id'] ?? '').toString() == messageId,
    );
    if (index < 0) {
      return false;
    }
    final updated = List<ChatMessage>.from(history);
    updated[index] = message;
    final shouldStickToBottom =
        scopeKey == _currentScopeKey && _isNearBottom(_scrollController);
    _historyByScope[scopeKey] = updated;
    if (mounted) {
      setState(() {
        if (updateStatus != null) {
          _status = updateStatus;
        }
      });
    }
    _historyPersistence.schedule(scopeKey);
    if (shouldStickToBottom) {
      _scrollToBottom(animated: false);
    }
    return true;
  }

  void _bufferCodegenStreamUpdate({
    required String scopeKey,
    required String messageId,
    required ChatMessage message,
  }) {
    final streamState = _codegenStreamStates.putIfAbsent(
      messageId,
      () => _CodegenStreamState(
        scopeKey: scopeKey,
        streamMessageId: messageId,
        latestMessage: message,
      ),
    );
    streamState.scopeKey = scopeKey;
    streamState.latestMessage = message;
    streamState.finalSeen =
        streamState.finalSeen || message.meta?['final'] == true;

    final nextContent = _normalizeCodegenStreamContent(message.content);
    final delta = _extractCodegenStreamDelta(
      streamState.fullContent,
      nextContent,
    );
    streamState.fullContent = nextContent;
    if (delta.isEmpty && !streamState.finalSeen) {
      return;
    }
    streamState.pendingDelta += delta;
    _pendingCodegenStreamIds.add(messageId);

    _streamFlushTimer ??= Timer.periodic(_streamFlushInterval, (_) {
      _flushCodegenStreamUpdates();
    });
  }

  void _flushCodegenStreamUpdates() {
    if (_pendingCodegenStreamIds.isEmpty) {
      _streamFlushTimer?.cancel();
      _streamFlushTimer = null;
      return;
    }
    if (!mounted) {
      return;
    }

    final streamIds = List<String>.from(_pendingCodegenStreamIds);
    _pendingCodegenStreamIds.clear();

    bool anyUpdated = false;
    String? lastUpdatedScopeKey;
    for (final streamId in streamIds) {
      final state = _codegenStreamStates[streamId];
      if (state == null) {
        continue;
      }
      final delta = state.pendingDelta;
      state.pendingDelta = '';
      if (delta.isNotEmpty &&
          _appendCodegenStreamDeltaToHistory(state, delta)) {
        anyUpdated = true;
        lastUpdatedScopeKey = state.scopeKey;
      }
      if (state.finalSeen && state.pendingDelta.isEmpty) {
        _codegenStreamStates.remove(streamId);
      }
    }

    final updatedCurrentScope = lastUpdatedScopeKey == _currentScopeKey;
    if (anyUpdated && updatedCurrentScope) {
      setState(() {});
      if (_isNearBottom(_scrollController)) {
        _scrollToBottom(animated: false);
      }
    }
  }

  String _normalizeCodegenStreamContent(String content) {
    return content
        .replaceAll('\r\n', '\n')
        .replaceAll('\r', '\n')
        .replaceAll('\u00a0', ' ');
  }

  String _extractCodegenStreamDelta(String previous, String current) {
    if (current.isEmpty || current == previous) {
      return '';
    }
    if (previous.isEmpty) {
      return current;
    }
    if (current.startsWith(previous)) {
      return current.substring(previous.length);
    }

    final overlap = _codegenStreamOverlap(previous, current);
    if (overlap > 0) {
      return current.substring(overlap);
    }

    // 后端异常回退成非累计快照时，另起一行追加，避免覆盖已展示内容。
    return '\n$current';
  }

  int _codegenStreamOverlap(String previous, String current) {
    final max = math.min(math.min(previous.length, current.length), 4096);
    for (var size = max; size > 0; size--) {
      if (previous.substring(previous.length - size) ==
          current.substring(0, size)) {
        return size;
      }
    }
    return 0;
  }

  bool _appendCodegenStreamDeltaToHistory(
    _CodegenStreamState state,
    String delta,
  ) {
    var history = List<ChatMessage>.from(
      _historyByScope[state.scopeKey] ?? <ChatMessage>[],
    );
    var remaining = delta;
    var changed = false;

    while (remaining.isNotEmpty) {
      var activeIndex = state.activeSegmentMessageId == null
          ? -1
          : history.lastIndexWhere(
              (item) =>
                  (item.meta?['_message_id'] ?? '').toString() ==
                  state.activeSegmentMessageId,
            );
      if (activeIndex < 0 ||
          history[activeIndex].content.length >= _codegenStreamSegmentLimit) {
        final nextMessage = _newCodegenStreamSegmentMessage(state);
        history = <ChatMessage>[...history, nextMessage];
        activeIndex = history.length - 1;
      }

      final active = history[activeIndex];
      final available = _codegenStreamSegmentLimit - active.content.length;
      if (available <= 0) {
        state.activeSegmentMessageId = null;
        continue;
      }
      final slice = _takeCodegenStreamSlice(remaining, available);
      final updatedContent = active.content + slice;
      history[activeIndex] = ChatMessage(
        content: updatedContent,
        direction: active.direction,
        timestamp: active.timestamp,
        status: active.status,
        scopeKey: active.scopeKey,
        authorId: active.authorId,
        groupId: active.groupId,
        messageType: active.messageType,
        meta: active.meta,
      );
      _appendProcessEntryToActiveCodegenHistory(
        history[activeIndex],
        slice,
        origin: 'codegen-stream',
        sessionId: (state.latestMessage.meta?['session_id'] ?? '').toString(),
      );
      remaining = remaining.substring(slice.length);
      changed = true;
      if (updatedContent.length >= _codegenStreamSegmentLimit) {
        state.activeSegmentMessageId = null;
      }
    }

    if (changed) {
      _historyByScope[state.scopeKey] = history;
      _historyPersistence.schedule(state.scopeKey);
    }
    return changed;
  }

  ChatMessage _newCodegenStreamSegmentMessage(_CodegenStreamState state) {
    state.segmentIndex += 1;
    final segmentMessageId =
        '${state.streamMessageId}::segment::${state.segmentIndex}';
    state.activeSegmentMessageId = segmentMessageId;
    final meta = <String, dynamic>{
      ...?state.latestMessage.meta,
      '_message_id': segmentMessageId,
      'stream_message_id': state.streamMessageId,
      'stream_segment_index': state.segmentIndex,
      'stream_segmented': true,
    };
    return ChatMessage(
      content: '',
      direction: state.latestMessage.direction,
      timestamp: state.latestMessage.timestamp,
      scopeKey: state.scopeKey,
      authorId: state.latestMessage.authorId,
      groupId: state.latestMessage.groupId,
      messageType: state.latestMessage.messageType,
      meta: meta,
    );
  }

  String _takeCodegenStreamSlice(String text, int maxLength) {
    if (text.length <= maxLength) {
      return text;
    }
    final newlineIndex = text.lastIndexOf('\n', maxLength);
    if (newlineIndex > maxLength ~/ 2) {
      return text.substring(0, newlineIndex + 1);
    }
    return text.substring(0, maxLength);
  }

  bool _isNearBottom(ScrollController controller, {double threshold = 120}) {
    if (!controller.hasClients) {
      return true;
    }
    return controller.position.extentAfter <= threshold;
  }

  void _scrollToBottom({bool animated = true}) {
    if (_scrollToBottomScheduled) {
      return;
    }
    _scrollToBottomScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _scrollToBottomScheduled = false;
      if (!_scrollController.hasClients) {
        return;
      }
      final position = _scrollController.position;
      final target = math.max(
        position.minScrollExtent,
        position.maxScrollExtent,
      );
      if ((position.pixels - target).abs() < 1) {
        return;
      }
      if (animated) {
        _scrollController.animateTo(
          target,
          duration: const Duration(milliseconds: 180),
          curve: Curves.easeOut,
        );
        return;
      }
      _scrollController.jumpTo(target);
    });
  }

  String _lastReadAtStorageKey(String scopeKey) =>
      '$_lastReadAtStoragePrefix::${_userIdController.text.trim()}::$scopeKey';

  String _messageAnchorId(ChatMessage message) {
    final stableMessageId = (message.meta?['_message_id'] ?? '')
        .toString()
        .trim();
    if (stableMessageId.isNotEmpty) {
      return '${message.scopeKey}|$stableMessageId';
    }
    final origin = (message.meta?['origin'] ?? '').toString().trim();
    final sessionId = (message.meta?['session_id'] ?? '').toString().trim();
    if (origin == 'codegen-stream' && sessionId.isNotEmpty) {
      return '${message.scopeKey}|$origin|$sessionId';
    }
    return [
      message.scopeKey,
      message.timestamp.microsecondsSinceEpoch.toString(),
      message.direction.name,
      message.authorId,
      message.groupId,
      message.messageType,
    ].join('|');
  }

  Key _messageRowKey(ChatMessage message) {
    final anchorId = _messageAnchorId(message);
    if (_activeMessageAnchorId == anchorId) {
      return _activeMessageAnchorKey;
    }
    return ValueKey<String>('message::$anchorId');
  }

  void _activateMessageAnchor(String anchorId) {
    if (_activeMessageAnchorId == anchorId) {
      return;
    }
    if (!mounted) {
      _activeMessageAnchorId = anchorId;
      return;
    }
    setState(() {
      _activeMessageAnchorId = anchorId;
    });
  }

  void _clearMessageAnchor(String anchorId) {
    if (_activeMessageAnchorId != anchorId) {
      return;
    }
    if (!mounted) {
      _activeMessageAnchorId = null;
      return;
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || _activeMessageAnchorId != anchorId) {
        return;
      }
      setState(() {
        _activeMessageAnchorId = null;
      });
    });
  }

  Future<int> _readLastReadAtMs(String scopeKey) async {
    final userId = _userIdController.text.trim();
    if (userId.isEmpty) {
      return 0;
    }
    final prefs = await SharedPreferences.getInstance();
    return prefs.getInt(_lastReadAtStorageKey(scopeKey)) ?? 0;
  }

  Future<void> _persistLastReadAtMs(String scopeKey, int timestampMs) async {
    final userId = _userIdController.text.trim();
    if (userId.isEmpty || timestampMs <= 0) {
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt(_lastReadAtStorageKey(scopeKey), timestampMs);
  }

  ChatMessage? _firstUnreadMessageForScope(String scopeKey, int lastReadAtMs) {
    final history = _historyByScope[scopeKey];
    if (history == null || history.isEmpty) {
      return null;
    }
    for (final message in history) {
      if (message.direction != MessageDirection.incoming) {
        continue;
      }
      if (message.timestamp.millisecondsSinceEpoch > lastReadAtMs) {
        return message;
      }
    }
    return null;
  }

  Future<void> _scrollToMessage(
    ChatMessage message, {
    bool animated = false,
    int attempts = 0,
  }) async {
    if (!mounted) {
      return;
    }
    final anchorId = _messageAnchorId(message);
    _activateMessageAnchor(anchorId);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final anchorContext = _activeMessageAnchorId == anchorId
          ? _activeMessageAnchorKey.currentContext
          : null;
      if (anchorContext == null) {
        if (attempts >= 6) {
          _clearMessageAnchor(anchorId);
          _scrollToBottom(animated: animated);
          return;
        }
        unawaited(
          Future<void>.delayed(const Duration(milliseconds: 16), () {
            return _scrollToMessage(
              message,
              animated: animated,
              attempts: attempts + 1,
            );
          }),
        );
        return;
      }
      Scrollable.ensureVisible(
        anchorContext,
        duration: animated ? const Duration(milliseconds: 180) : Duration.zero,
        curve: Curves.easeOut,
        alignment: 0.06,
      );
      _clearMessageAnchor(anchorId);
    });
  }

  Future<void> _scrollToUnreadOrBottom(String scopeKey) async {
    final lastReadAtMs = await _readLastReadAtMs(scopeKey);
    final unreadMessage = _firstUnreadMessageForScope(scopeKey, lastReadAtMs);
    if (unreadMessage != null) {
      await _scrollToMessage(unreadMessage);
      return;
    }
    _scrollToBottom(animated: false);
  }

  Future<void> _revealScopeEntryPoint(String scopeKey) async {
    await _scrollToUnreadOrBottom(scopeKey);
    await _markScopeAsRead(scopeKey);
  }

  Future<void> _markScopeAsRead(String scopeKey) async {
    final history = _historyByScope[scopeKey];
    if (history == null || history.isEmpty) {
      return;
    }
    var latestIncomingAtMs = 0;
    for (final message in history) {
      if (message.direction != MessageDirection.incoming) {
        continue;
      }
      final timestampMs = message.timestamp.millisecondsSinceEpoch;
      if (timestampMs > latestIncomingAtMs) {
        latestIncomingAtMs = timestampMs;
      }
    }
    if (latestIncomingAtMs <= 0) {
      return;
    }
    await _persistLastReadAtMs(scopeKey, latestIncomingAtMs);
  }

  String _describeRequestError(Object err, {required String operation}) {
    if (err is TimeoutException) {
      if (operation == 'Download attachment') {
        return '$operation timed out. app-agent did not finish within 30 seconds.';
      }
      return '$operation timed out. app-agent did not respond within 8 seconds.';
    }
    if (err is SocketException) {
      return '$operation failed: unable to reach app-agent.';
    }
    if (err is AppAgentUnauthorizedException) {
      return '$operation failed: login required or expired.';
    }
    final raw = err.toString();
    if (raw.startsWith('ClientException: ')) {
      return '$operation failed: ${raw.substring('ClientException: '.length)}';
    }
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
    if (message.messageType == 'video') {
      final meta = Map<String, dynamic>.from(message.meta ?? const {});
      var videoPath = (meta['file_path'] ?? '').toString().trim();
      if (videoPath.isEmpty || !await File(videoPath).exists()) {
        final hydrated = await _hydrateIncomingMediaMeta(
          messageType: 'video',
          meta: meta,
        );
        videoPath = (hydrated['file_path'] ?? '').toString().trim();
        if (videoPath.isNotEmpty) {
          await _updateMessageMeta(message, <String, dynamic>{
            'file_path': videoPath,
          });
        }
      }
      if (videoPath.isEmpty || !await File(videoPath).exists()) {
        _appendSystem('Video file unavailable for playback.');
      }
      return;
    }

    if (message.messageType == 'audio') {
      final meta = Map<String, dynamic>.from(message.meta ?? const {});
      var audioPath = (meta['audio_path'] ?? '').toString().trim();
      if (audioPath.isEmpty || !await File(audioPath).exists()) {
        final hydrated = await _hydrateIncomingMediaMeta(
          messageType: 'audio',
          meta: meta,
        );
        audioPath = (hydrated['audio_path'] ?? '').toString().trim();
        if (audioPath.isNotEmpty) {
          await _updateMessageMeta(message, <String, dynamic>{
            'audio_path': audioPath,
          });
        }
      }
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
      return;
    }

    if (!isApkChatMessage(message)) {
      return;
    }
    late final String apkPath;
    try {
      apkPath = await _resolveOrDownloadApkPath(message);
    } catch (err) {
      if (err is StateError) {
        _appendSystem(err.message.toString());
        return;
      }
      _appendSystem(
        _describeRequestError(err, operation: 'Download attachment'),
      );
      return;
    }
    try {
      await _installDownloadedApk(apkPath);
    } catch (err) {
      _appendSystem('安装 APK 失败：$err');
    }
  }

  String _groupScopeKey(String groupId) => 'group:${groupId.toLowerCase()}';

  String _historyStorageKey(String scopeKey) =>
      '$_historyStoragePrefix${_userIdController.text.trim()}::$scopeKey';

  String _historyBackupStorageKey(String scopeKey) =>
      '$_historyBackupStoragePrefix${_userIdController.text.trim()}::$scopeKey';

  String? _extractScopeKey(String key, String userId, String prefix) {
    final normalizedUserId = userId.trim();
    final normalizedPrefix = '$prefix$normalizedUserId::';
    if (!key.startsWith(normalizedPrefix)) {
      return null;
    }
    final scopeKey = key.substring(normalizedPrefix.length).trim();
    return scopeKey.isEmpty ? null : scopeKey;
  }

  Future<String> _readPersistedHistoryRaw(
    SharedPreferences prefs,
    String scopeKey,
  ) async {
    final primary = prefs.getString(_historyStorageKey(scopeKey)) ?? '';
    if (primary.isNotEmpty) {
      return primary;
    }
    try {
      return (await _secureStorage.read(
                key: _historyBackupStorageKey(scopeKey),
              ) ??
              '')
          .trim();
    } catch (err) {
      debugPrint('Read history backup failed for $scopeKey: $err');
      return '';
    }
  }

  Future<void> _loadHistory(String scopeKey) async {
    final userId = _userIdController.text.trim();
    if (userId.isEmpty) {
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    final raw = await _readPersistedHistoryRaw(prefs, scopeKey);
    if (raw.isEmpty) {
      _historyByScope[scopeKey] = <ChatMessage>[];
      if (mounted) {
        setState(() {});
        if (scopeKey == _currentScopeKey) {
          unawaited(_revealScopeEntryPoint(scopeKey));
        }
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
        if (scopeKey == _currentScopeKey) {
          unawaited(_revealScopeEntryPoint(scopeKey));
        }
      }
    } catch (_) {
      _historyByScope[scopeKey] = <ChatMessage>[];
    }
  }

  Future<void> _loadAllHistoryForUser() async {
    final userId = _userIdController.text.trim();
    if (userId.isEmpty) {
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    final scopeKeys = <String>{'direct'};

    for (final key in prefs.getKeys()) {
      final scopeKey = _extractScopeKey(key, userId, _historyStoragePrefix);
      if (scopeKey != null) {
        scopeKeys.add(scopeKey);
      }
    }

    try {
      final secureEntries = await _secureStorage.readAll();
      for (final key in secureEntries.keys) {
        final scopeKey = _extractScopeKey(
          key,
          userId,
          _historyBackupStoragePrefix,
        );
        if (scopeKey != null) {
          scopeKeys.add(scopeKey);
        }
      }
    } catch (err) {
      debugPrint('Enumerate secure history backup failed: $err');
    }

    for (final scopeKey in scopeKeys) {
      await _loadHistory(scopeKey);
    }
    if (!_historyByScope.containsKey('direct')) {
      _historyByScope['direct'] = <ChatMessage>[];
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
    final encoded = jsonEncode(data);
    await prefs.setString(_historyStorageKey(scopeKey), encoded);
    try {
      await _secureStorage.write(
        key: _historyBackupStorageKey(scopeKey),
        value: encoded,
      );
    } catch (err) {
      debugPrint('Persist history backup failed for $scopeKey: $err');
    }
  }

  Future<void> _switchToDirectScope() async {
    _currentGroupId = '';
    _groupTabsExpanded = false;
    await _loadHistory(_currentScopeKey);
    if (mounted) {
      setState(() {});
    }
  }

  Future<void> _switchToGroupScope(
    String groupId, {
    bool keepTabsExpanded = false,
  }) async {
    _currentGroupId = groupId;
    _groupTabsExpanded = _groups.length > 1 && keepTabsExpanded;
    await _loadHistory(_currentScopeKey);
    if (mounted) {
      setState(() {});
    }
  }

  Future<void> _switchToGroupsTab() async {
    if (_groups.isEmpty) {
      return;
    }
    final targetGroupId = _resolvePreferredGroupId(
      _groups,
      preferredGroupId: _currentGroupId,
    );
    if (_currentGroupId.isEmpty) {
      await _switchToGroupScope(
        targetGroupId.isEmpty ? _groups.first.id : targetGroupId,
        keepTabsExpanded: _groups.length > 1,
      );
      return;
    }
    if (_groups.length <= 1) {
      await _switchToGroupScope(targetGroupId);
      return;
    }
    if (!mounted) {
      _groupTabsExpanded = !_groupTabsExpanded;
      return;
    }
    setState(() {
      _groupTabsExpanded = !_groupTabsExpanded;
    });
  }

  Future<void> _refreshGroups() async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      return;
    }
    try {
      final groups = await _runAuthed(
        'Load groups',
        (client) => client.listGroups(),
      );
      final previousGroupId = _currentGroupId;
      final previousTabsExpanded = _groupTabsExpanded;
      final nextGroupId = _resolvePreferredGroupId(
        groups,
        preferredGroupId: _currentGroupId,
        allowImplicitSingleSelection: false,
      );
      if (mounted) {
        setState(() {
          _groups
            ..clear()
            ..addAll(groups);
          _currentGroupId = nextGroupId;
          _groupTabsExpanded =
              groups.length > 1 &&
              nextGroupId.isNotEmpty &&
              previousTabsExpanded;
        });
      }
      if (nextGroupId != previousGroupId || nextGroupId.isNotEmpty) {
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
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    try {
      final groups = await _runAuthed(
        'Group $action',
        (client) => client.mutateGroup(action, groupId),
      );
      final previousGroupId = _currentGroupId;
      final previousTabsExpanded = _groupTabsExpanded;
      final nextGroupId = action == 'leave'
          ? _resolvePreferredGroupId(
              groups,
              preferredGroupId: _currentGroupId == groupId
                  ? null
                  : _currentGroupId,
            )
          : _resolvePreferredGroupId(groups, preferredGroupId: groupId);
      if (mounted) {
        setState(() {
          _groups
            ..clear()
            ..addAll(groups);
          _currentGroupId = nextGroupId;
          _groupTabsExpanded =
              groups.length > 1 &&
              nextGroupId.isNotEmpty &&
              previousTabsExpanded &&
              action == 'leave';
        });
      }
      if (nextGroupId != previousGroupId || nextGroupId.isNotEmpty) {
        await _loadHistory(_currentScopeKey);
      }
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
      final session = await _client.login();
      await _finishAuthenticatedLogin(
        session,
        successStatus: 'Login success, connecting WebSocket...',
      );
    } catch (err) {
      await _clearLocalAuthState(
        status: 'Login failed',
        clearStoredRefreshToken: false,
      );
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
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
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
      final socket = await _runAuthed(
        'WebSocket connect',
        (client) => client.connectWebSocket(),
      );
      await _socketSub?.cancel();
      await _socket?.sink.close();

      _socket = socket;
      _socketSub = socket.stream.listen(
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
      addFlutterClientLog('WebSocket 已连接');
      unawaited(_syncCortanaSettings(silent: true));
    } catch (err) {
      final errorText = _describeRequestError(
        err,
        operation: 'WebSocket connect',
      );
      addFlutterClientLog('WebSocket 连接失败: $errorText');
      if (mounted) {
        setState(() {
          _connecting = false;
          _connected = false;
          _status = errorText;
        });
      }
      _scheduleReconnect();
    }
  }

  Future<void> _disconnectWs() async {
    _autoReconnect = false;
    _reconnectTimer?.cancel();
    await _socketSub?.cancel();
    await _socket?.sink.close();
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

  Future<void> _onWsData(dynamic data) async {
    try {
      final text = data is String ? data : utf8.decode(data as List<int>);
      final decoded = jsonDecode(text) as Map<String, dynamic>;
      final envelope = PushEnvelope.fromJson(decoded);
      if (envelope.messageId.isNotEmpty) {
        _sendSocketAck(envelope.messageId);
      }
      final meta = envelope.meta ?? <String, dynamic>{};
      final origin = (meta['origin'] ?? '').toString();
      final groupId = (meta['group_id'] ?? '').toString().trim();
      final isGroupMessage = groupId.isNotEmpty;
      if (!isGroupMessage &&
          envelope.userId.isNotEmpty &&
          envelope.userId != _userIdController.text.trim()) {
        return;
      }
      if (envelope.sequence > 0 && envelope.sequence <= _lastSequence) {
        return;
      }
      if (envelope.sequence > 0) {
        _lastSequence = envelope.sequence;
      }

      if (origin == 'llm-debug' || meta['app_debug'] == true) {
        _recordLlmDebugEvent(envelope, meta);
        return;
      }

      if (_shouldFilterIncomingEnvelope(
        envelope: envelope,
        meta: meta,
        isGroupMessage: isGroupMessage,
      )) {
        return;
      }

      // 检测 Cortana 播报消息
      if (envelope.messageType == 'cortana_broadcast' ||
          origin == 'cortana-agent') {
        _handleCortanaBroadcast(envelope, meta);
        return;
      }

      final when = DateTime.fromMillisecondsSinceEpoch(envelope.timestamp);
      final resolvedMeta = await _hydrateIncomingMediaMeta(
        messageType: envelope.messageType,
        meta: meta,
      );
      final messageMeta = Map<String, dynamic>.from(resolvedMeta);
      if (envelope.messageId.isNotEmpty) {
        messageMeta['_message_id'] = envelope.messageId;
      }
      final scopeKey = groupId.isEmpty ? 'direct' : _groupScopeKey(groupId);
      final fromUser = (resolvedMeta['from_user'] ?? '').toString().trim();
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

      final chatMessage = ChatMessage(
        content: envelope.content,
        direction: direction,
        timestamp: when,
        scopeKey: scopeKey,
        authorId: fromUser,
        groupId: groupId,
        messageType: envelope.messageType,
        meta: messageMeta,
      );
      if (envelope.messageId.isNotEmpty) {
        final origin = (meta['origin'] ?? '').toString();
        if (origin == 'codegen-stream') {
          _bufferCodegenStreamUpdate(
            scopeKey: scopeKey,
            messageId: envelope.messageId,
            message: chatMessage,
          );
          _seenMessageIds.add(envelope.messageId);
          return;
        }
      }
      if (envelope.messageId.isNotEmpty &&
          _replaceMessageById(
            scopeKey: scopeKey,
            messageId: envelope.messageId,
            message: chatMessage,
            updateStatus: isSystemMessage
                ? envelope.content
                : 'Received message',
          )) {
        _recordIncomingProcessMessage(chatMessage);
        _maybePresentCortanaReplyFromIncoming(chatMessage);
        _seenMessageIds.add(envelope.messageId);
        return;
      }

      _appendMessage(
        chatMessage,
        updateStatus: isSystemMessage ? envelope.content : 'Received message',
      );
      _recordIncomingProcessMessage(chatMessage);
      _maybePresentCortanaReplyFromIncoming(chatMessage);
      if (!isSystemMessage &&
          direction == MessageDirection.incoming &&
          envelope.messageType == 'file') {
        final autoInstallMessage = ChatMessage(
          content: envelope.content,
          direction: direction,
          timestamp: when,
          scopeKey: scopeKey,
          authorId: fromUser,
          groupId: groupId,
          messageType: envelope.messageType,
          meta: messageMeta,
        );
        if (isApkChatMessage(autoInstallMessage) &&
            envelope.messageId.isNotEmpty &&
            !_autoInstallTriggered.contains(envelope.messageId)) {
          final apkPath = (resolvedMeta['file_path'] ?? '').toString().trim();
          if (apkPath.isNotEmpty) {
            _autoInstallTriggered.add(envelope.messageId);
            unawaited(_installDownloadedApk(apkPath));
          }
        }
      }
      if (envelope.messageId.isNotEmpty) {
        _seenMessageIds.add(envelope.messageId);
      }
    } catch (err) {
      if (mounted) {
        setState(() {
          _status = 'Invalid WebSocket payload';
        });
      }
    }
  }

  void _recordLlmDebugEvent(PushEnvelope envelope, Map<String, dynamic> meta) {
    final eventName = (meta['debug_event'] ?? '').toString().trim();
    final item = LlmDebugEvent(
      event: eventName.isEmpty ? 'debug' : eventName,
      content: envelope.content,
      timestamp: DateTime.fromMillisecondsSinceEpoch(envelope.timestamp),
      meta: Map<String, dynamic>.from(meta),
    );
    if (!mounted) {
      _llmDebugEvents.add(item);
      if (_llmDebugEvents.length > 300) {
        _llmDebugEvents.removeRange(0, _llmDebugEvents.length - 300);
      }
      return;
    }
    setState(() {
      _llmDebugEvents.add(item);
      if (_llmDebugEvents.length > 300) {
        _llmDebugEvents.removeRange(0, _llmDebugEvents.length - 300);
      }
      _status = 'LLM debug: ${item.label}';
    });
  }

  void _sendSocketAck(String messageId) {
    final socket = _socket;
    if (socket == null || messageId.trim().isEmpty) {
      return;
    }
    try {
      socket.sink.add(
        jsonEncode(<String, dynamic>{'type': 'ack', 'message_id': messageId}),
      );
    } catch (_) {}
  }

  void _handleSocketClosed(String text) {
    _socketSub = null;
    _socket = null;
    addFlutterClientLog('WebSocket 断开: $text');
    if (mounted) {
      setState(() {
        _connecting = false;
        _connected = false;
        _status = text;
      });
    }
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

  bool _shouldFilterIncomingEnvelope({
    required PushEnvelope envelope,
    required Map<String, dynamic> meta,
    required bool isGroupMessage,
  }) {
    final messageType = envelope.messageType.trim().toLowerCase();
    final origin = (meta['origin'] ?? '').toString().trim().toLowerCase();
    final content = envelope.content.trim();
    final isAppProcess = meta['app_process'] == true || origin == 'app-process';

    if (isGroupMessage) {
      return messageType == 'system';
    }

    if (messageType == 'system' && !isAppProcess) {
      return true;
    }

    if (isAppProcess && !_hasPendingCodegenHistoryExecution()) {
      return true;
    }

    if (!isAppProcess &&
        origin == 'llm-agent' &&
        _looksLikeStatusMessage(content)) {
      return true;
    }

    return false;
  }

  bool _looksLikeStatusMessage(String content) {
    if (content.isEmpty) {
      return false;
    }
    const prefixes = <String>[
      '[system]',
      '[tool]',
      '[result]',
      '[error]',
      'Codegen task completed',
      'Codegen task failed',
      'App Agent status',
      'Gateway disconnected',
      'WebSocket connected.',
    ];
    for (final prefix in prefixes) {
      if (content.startsWith(prefix)) {
        return true;
      }
    }
    return false;
  }

  Future<Map<String, dynamic>> _hydrateIncomingMediaMeta({
    required String messageType,
    required Map<String, dynamic> meta,
  }) async {
    if (meta.isEmpty) {
      return meta;
    }
    final fileId = (meta['file_id'] ?? '').toString().trim();
    if (fileId.isEmpty) {
      return meta;
    }
    final resolved = Map<String, dynamic>.from(meta);
    try {
      switch (messageType.trim().toLowerCase()) {
        case 'audio':
          final currentPath = (resolved['audio_path'] ?? '').toString().trim();
          if (currentPath.isNotEmpty && await File(currentPath).exists()) {
            return resolved;
          }
          final extension =
              (resolved['audio_format'] ?? resolved['file_format'] ?? 'bin')
                  .toString()
                  .trim();
          final audioPath = await _attachmentPathForFileID(
            fileId: fileId,
            subdir: 'voice_messages',
            prefix: 'voice',
            extension: extension.isEmpty ? 'bin' : extension,
          );
          final existingFile = File(audioPath);
          if (!await existingFile.exists()) {
            await _runAuthed('Download attachment', (client) {
              return client.downloadAttachmentToFile(
                fileId,
                destinationPath: audioPath,
                attachmentMeta: resolved,
                onProgress: (receivedBytes, totalBytes, resumed) {
                  _updateDownloadStatus(
                    label: '语音',
                    receivedBytes: receivedBytes,
                    totalBytes: totalBytes,
                    resumed: resumed,
                  );
                },
              );
            });
          }
          _clearDownloadStatus(successText: '语音下载完成');
          resolved['audio_path'] = audioPath;
          return resolved;
        case 'image':
          final currentPath = (resolved['image_path'] ?? '').toString().trim();
          if (currentPath.isNotEmpty && await File(currentPath).exists()) {
            return resolved;
          }
          if ((resolved['image_base64'] ?? '').toString().trim().isNotEmpty) {
            return resolved;
          }
          final fileName = (resolved['file_name'] ?? '').toString().trim();
          final imageExtension = _resolveFileExtension(
            fileName: fileName,
            fileFormat: (resolved['image_format'] ?? '').toString(),
          );
          final imagePath = await _attachmentPathForFileID(
            fileId: fileId,
            subdir: 'downloads',
            prefix: 'image',
            extension: imageExtension,
          );
          final imageFile = File(imagePath);
          if (!await imageFile.exists()) {
            await _runAuthed('Download attachment', (client) {
              return client.downloadAttachmentToFile(
                fileId,
                destinationPath: imagePath,
                attachmentMeta: resolved,
                onProgress: (receivedBytes, totalBytes, resumed) {
                  _updateDownloadStatus(
                    label: fileName.isEmpty ? '图片' : fileName,
                    receivedBytes: receivedBytes,
                    totalBytes: totalBytes,
                    resumed: resumed,
                  );
                },
              );
            });
          }
          _clearDownloadStatus(successText: '图片下载完成');
          resolved['image_path'] = imagePath;
          return resolved;
        case 'file':
        case 'archive':
        case 'video':
          final currentPath = (resolved['file_path'] ?? '').toString().trim();
          if (currentPath.isNotEmpty && await File(currentPath).exists()) {
            return resolved;
          }
          final fileName = (resolved['file_name'] ?? '').toString().trim();
          final extension = _resolveFileExtension(
            fileName: fileName,
            fileFormat: (resolved['file_format'] ?? '').toString(),
          );
          final filePath = await _attachmentPathForFileID(
            fileId: fileId,
            subdir: 'downloads',
            prefix: 'file',
            extension: extension,
          );
          final file = File(filePath);
          if (!await file.exists()) {
            await _runAuthed('Download attachment', (client) {
              return client.downloadAttachmentToFile(
                fileId,
                destinationPath: filePath,
                attachmentMeta: resolved,
                onProgress: (receivedBytes, totalBytes, resumed) {
                  _updateDownloadStatus(
                    label: fileName.isEmpty ? '附件' : fileName,
                    receivedBytes: receivedBytes,
                    totalBytes: totalBytes,
                    resumed: resumed,
                  );
                },
              );
            });
          }
          _clearDownloadStatus(successText: '附件下载完成');
          resolved['file_path'] = filePath;
          return resolved;
        default:
          return resolved;
      }
    } catch (err) {
      _clearDownloadStatus();
      _appendSystem(
        _describeRequestError(err, operation: 'Download attachment'),
      );
      return resolved;
    }
  }

  String _resolveFileExtension({
    required String fileName,
    required String fileFormat,
  }) {
    final trimmedName = fileName.trim();
    final dot = trimmedName.lastIndexOf('.');
    if (dot >= 0 && dot < trimmedName.length - 1) {
      return trimmedName.substring(dot + 1).trim().toLowerCase();
    }
    final format = fileFormat.trim().toLowerCase();
    return format.isEmpty ? 'bin' : format;
  }

  Future<String> _resolveOrDownloadApkPath(ChatMessage message) async {
    final meta = message.meta ?? const <String, dynamic>{};
    final currentPath = (meta['file_path'] ?? '').toString().trim();
    if (currentPath.isNotEmpty && await File(currentPath).exists()) {
      return currentPath;
    }

    final fileId = (meta['file_id'] ?? '').toString().trim();
    if (fileId.isEmpty) {
      throw StateError('APK 下载信息缺失，无法安装。');
    }

    final fileName = (meta['file_name'] ?? '').toString().trim();
    final newVersion = extractApkVersionFromString(fileName);

    // Log version info for debugging
    if (newVersion != null) {
      _appendSystem('收到 APK: $fileName (版本 $newVersion)');
    } else {
      _appendSystem('收到 APK: $fileName');
    }

    final extension = _resolveFileExtension(
      fileName: fileName,
      fileFormat: (meta['file_format'] ?? '').toString(),
    );
    final apkPath = await _attachmentPathForFileID(
      fileId: fileId,
      subdir: 'downloads',
      prefix: 'file',
      extension: extension,
    );
    final apkFile = File(apkPath);

    // Check if file already exists locally and verify its size
    if (await apkFile.exists()) {
      final existingSize = await apkFile.length();
      final expectedSize = (meta['file_size'] is int)
          ? meta['file_size'] as int
          : (meta['file_size'] is String
                ? int.tryParse(meta['file_size'] as String)
                : null);

      if (expectedSize != null && existingSize >= expectedSize) {
        // File is complete
        _appendSystem('APK 文件已存在且完整: $apkPath (${_formatBytes(existingSize)})');
        await _updateMessageMeta(message, <String, dynamic>{
          'file_path': apkPath,
        });
        return apkPath;
      } else {
        // File is incomplete, delete it and re-download
        _appendSystem(
          'APK 文件不完整 (已有 ${_formatBytes(existingSize)}${expectedSize != null ? ' / ${_formatBytes(expectedSize)}' : ''})，删除后重新下载',
        );
        try {
          await apkFile.delete();
        } catch (_) {}
      }
    }

    // Download the APK
    try {
      await _runAuthed('Download attachment', (client) {
        return client.downloadAttachmentToFile(
          fileId,
          destinationPath: apkPath,
          attachmentMeta: meta,
          onProgress: (receivedBytes, totalBytes, resumed) {
            _updateDownloadStatus(
              label: fileName.isEmpty ? 'APK' : fileName,
              receivedBytes: receivedBytes,
              totalBytes: totalBytes,
              resumed: resumed,
            );
          },
        );
      });
      // Try to extract version from filename (e.g., app-release-1.0.0.apk -> 1.0.0)
      String versionLabel = '';
      final versionMatch = RegExp(
        r'[-_](\d+\.\d+\.\d+(?:\+\d+)?)[^.]*\.apk$',
        caseSensitive: false,
      ).firstMatch(fileName);
      if (versionMatch != null) {
        versionLabel = ' v${versionMatch.group(1)}';
      }
      _clearDownloadStatus(successText: 'APK 下载完成$versionLabel');
    } catch (err) {
      _clearDownloadStatus();
      _appendSystem('APK 下载失败: $err');
      rethrow;
    }

    await _updateMessageMeta(message, <String, dynamic>{'file_path': apkPath});
    return apkPath;
  }

  Future<void> _updateMessageMeta(
    ChatMessage target,
    Map<String, dynamic> patch,
  ) async {
    if (patch.isEmpty) {
      return;
    }
    final history = _historyByScope[target.scopeKey];
    if (history == null || history.isEmpty) {
      return;
    }

    var matchedIndex = -1;
    for (var i = history.length - 1; i >= 0; i--) {
      final candidate = history[i];
      if (_isSameStoredMessage(candidate, target)) {
        matchedIndex = i;
        break;
      }
    }
    if (matchedIndex < 0) {
      return;
    }

    final existing = history[matchedIndex];
    final mergedMeta = <String, dynamic>{
      if (existing.meta != null) ...existing.meta!,
      ...patch,
    };
    final updated = ChatMessage(
      content: existing.content,
      direction: existing.direction,
      timestamp: existing.timestamp,
      status: existing.status,
      scopeKey: existing.scopeKey,
      authorId: existing.authorId,
      groupId: existing.groupId,
      messageType: existing.messageType,
      meta: mergedMeta,
    );
    final updatedHistory = List<ChatMessage>.from(history);
    updatedHistory[matchedIndex] = updated;
    _historyByScope[target.scopeKey] = updatedHistory;

    if (mounted && target.scopeKey == _currentScopeKey) {
      setState(() {});
    }
    _historyPersistence.schedule(target.scopeKey);
  }

  bool _isSameStoredMessage(ChatMessage a, ChatMessage b) {
    return a.timestamp.millisecondsSinceEpoch ==
            b.timestamp.millisecondsSinceEpoch &&
        a.content == b.content &&
        a.direction == b.direction &&
        a.messageType == b.messageType &&
        a.authorId == b.authorId &&
        a.groupId == b.groupId;
  }

  Future<void> _installDownloadedApk(String apkPath) async {
    if (!_isAndroidHost) {
      _appendSystem('APK 安装仅支持 Android 客户端。');
      return;
    }
    final resp = await _apkInstaller.installApk(apkPath);
    final status = (resp['status'] ?? '').toString().trim();
    if (status == 'permission_required') {
      _appendSystem('请先允许安装未知来源应用，然后再次点击 APK 安装。');
      return;
    }
    if (mounted) {
      setState(() {
        _status = '已发起 APK 安装';
      });
    }
    _appendSystem('APK 已下载，正在调用系统安装器。');
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
}
