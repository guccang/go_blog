// ignore_for_file: invalid_use_of_protected_member
part of '../../main.dart';

extension _ChatPageStateCortana on _ChatPageState {
  bool _isCortanaProgressMessage(ChatMessage msg) {
    if (msg.messageType != 'text') {
      return false;
    }
    final content = msg.content.trim();
    if (content.isEmpty) {
      return true;
    }
    const progressPrefixes = <String>[
      '收到消息',
      '思考:',
      '工具调用:',
      '工具结果:',
      '工具进度:',
      '子任务开始:',
      '子任务完成:',
      '子任务失败:',
      '子任务跳过:',
      '子任务结果:',
      '子任务思考:',
      '子任务回复:',
      '异步子任务:',
      '延后处理:',
      '任务完成:',
      '任务取消:',
      '强制总结:',
      '子任务超时:',
      '模型错误:',
      '进度:',
      '重试:',
      '修改:',
      '路由:',
      '技能开始:',
      '技能工具调用:',
      '技能完成:',
    ];
    return progressPrefixes.any(content.startsWith);
  }

  List<CortanaSuggestedReply> _extractCortanaSuggestedReplies(
    Map<String, dynamic>? actionPlan,
  ) {
    if (actionPlan == null || actionPlan.isEmpty) {
      return const <CortanaSuggestedReply>[];
    }
    final raw =
        actionPlan['suggested_replies'] ??
        actionPlan['reply_options'] ??
        actionPlan['options'];
    if (raw is! List) {
      return const <CortanaSuggestedReply>[];
    }
    final replies = <CortanaSuggestedReply>[];
    for (final item in raw) {
      final reply = item is Map
          ? CortanaSuggestedReply.fromMap(Map<String, dynamic>.from(item))
          : CortanaSuggestedReply(
              label: item.toString().trim(),
              message: item.toString().trim(),
            );
      if (reply.label.isEmpty) {
        continue;
      }
      replies.add(reply);
      if (replies.length >= 4) {
        break;
      }
    }
    return replies;
  }

  CortanaReplyPayload? _extractCortanaReplyPayload(ChatMessage msg) {
    if (msg.direction != MessageDirection.incoming) {
      return null;
    }
    final meta = msg.meta ?? const <String, dynamic>{};
    if (meta['cortana_broadcast'] == true) {
      return null;
    }
    if (msg.messageType == 'audio') {
      final audioPath = (meta['audio_path'] ?? meta['cortana_audio_path'] ?? '')
          .toString()
          .trim();
      final audioBytes = _decodeBase64OrNull(
        meta['audio_base64'] ?? meta['cortana_audio_base64'],
      );
      if (audioPath.isNotEmpty || audioBytes != null) {
        final rawActionPlan = meta['cortana_action_plan'];
        final actionPlan = rawActionPlan is Map
            ? Map<String, dynamic>.from(rawActionPlan)
            : null;
        return CortanaReplyPayload(
          text: (meta['speech_text'] ?? meta['cortana_text'] ?? msg.content)
              .toString()
              .trim(),
          audioPath: audioPath,
          audioBytes: audioBytes,
          audioFormat:
              (meta['audio_format'] ?? meta['cortana_audio_format'] ?? '')
                  .toString()
                  .trim(),
          actionPlan: actionPlan,
          suggestedReplies: _extractCortanaSuggestedReplies(actionPlan),
          requestId: (meta['cortana_request_id'] ?? '').toString().trim(),
        );
      }
    }
    if (msg.messageType == 'text' && !_isCortanaProgressMessage(msg)) {
      final rawActionPlan = meta['cortana_action_plan'];
      final actionPlan = rawActionPlan is Map
          ? Map<String, dynamic>.from(rawActionPlan)
          : null;
      return CortanaReplyPayload(
        text: (meta['speech_text'] ?? meta['cortana_text'] ?? msg.content)
            .toString()
            .trim(),
        actionPlan: actionPlan,
        suggestedReplies: _extractCortanaSuggestedReplies(actionPlan),
        requestId: (meta['cortana_request_id'] ?? '').toString().trim(),
      );
    }
    return null;
  }

  Uint8List? _decodeBase64OrNull(Object? value) {
    final raw = (value ?? '').toString().trim();
    if (raw.isEmpty) {
      return null;
    }
    try {
      return base64Decode(raw);
    } catch (_) {
      return null;
    }
  }

  String _buildCortanaRequestId() {
    final now = DateTime.now().microsecondsSinceEpoch;
    final userId = _userIdController.text.trim();
    return 'cortana_${userId}_$now';
  }

  String _buildCortanaReplyKey(ChatMessage msg) {
    final audioPath =
        (msg.meta?['audio_path'] ?? msg.meta?['cortana_audio_path'] ?? '')
            .toString()
            .trim();
    final audioBase64 =
        (msg.meta?['audio_base64'] ?? msg.meta?['cortana_audio_base64'] ?? '')
            .toString()
            .trim();
    final requestId = (msg.meta?['cortana_request_id'] ?? '').toString().trim();
    return [
      msg.timestamp.millisecondsSinceEpoch.toString(),
      msg.messageType,
      requestId,
      audioPath,
      audioBase64,
      msg.content.trim(),
    ].join('|');
  }

  List<CortanaReplayItem> _buildCortanaReplayHistory() {
    final replayItems = <CortanaReplayItem>[];
    for (final msg in _messages.toList().reversed) {
      if (msg.direction != MessageDirection.incoming ||
          msg.messageType != 'audio') {
        continue;
      }
      final meta = msg.meta ?? const <String, dynamic>{};
      final audioPath = (meta['audio_path'] ?? meta['cortana_audio_path'] ?? '')
          .toString()
          .trim();
      final audioBase64 =
          (meta['audio_base64'] ?? meta['cortana_audio_base64'] ?? '')
              .toString()
              .trim();
      final audioBytes = _decodeBase64OrNull(audioBase64);
      if (audioPath.isEmpty && audioBytes == null) {
        continue;
      }
      final rawActionPlan = meta['cortana_action_plan'];
      replayItems.add(
        CortanaReplayItem(
          id: _buildCortanaReplyKey(msg),
          text: (meta['speech_text'] ?? meta['cortana_text'] ?? msg.content)
              .toString()
              .trim(),
          audioPath: audioPath,
          audioBytes: audioBytes,
          audioFormat:
              (meta['audio_format'] ?? meta['cortana_audio_format'] ?? '')
                  .toString()
                  .trim(),
          createdAt: msg.timestamp,
          actionPlan: rawActionPlan is Map
              ? Map<String, dynamic>.from(rawActionPlan)
              : null,
          sourceLabel: '聊天页签',
          fileId: (meta['file_id'] ?? '').toString().trim(),
          storageProvider: (meta['storage_provider'] ?? '').toString().trim(),
          objectKey: (meta['object_key'] ?? '').toString().trim(),
        ),
      );
      if (replayItems.length >= 6) {
        break;
      }
    }
    return replayItems;
  }

  void _handleCortanaBroadcast(
    PushEnvelope envelope,
    Map<String, dynamic> meta,
  ) {
    final broadcastText = (meta['cortana_text'] ?? envelope.content)
        .toString()
        .trim();
    if (broadcastText.isEmpty) return;

    final expression = (meta['cortana_expression'] ?? 'happy')
        .toString()
        .trim();
    final motion = (meta['cortana_motion'] ?? 'IdleWave').toString().trim();
    final rawActionPlan = meta['cortana_action_plan'];
    final actionPlan = rawActionPlan is Map
        ? Map<String, dynamic>.from(rawActionPlan)
        : <String, dynamic>{
            'expression': expression,
            'motion': motion,
            'actions': <Map<String, dynamic>>[
              <String, dynamic>{'motion': motion, 'delay': 0},
            ],
          };

    // 提取 TTS 音频数据
    final audioPath = (meta['cortana_audio_path'] ?? '').toString().trim();
    final audioBase64 = (meta['cortana_audio_base64'] ?? '').toString().trim();
    final audioFormat = (meta['cortana_audio_format'] ?? '').toString().trim();
    final audioBytes = _decodeBase64OrNull(audioBase64);

    addFlutterClientLog('收到播报: $broadcastText');

    debugPrint(
      '[Cortana Broadcast] text=$broadcastText expr=$expression motion=$motion audioPath=${audioPath.isEmpty ? "none" : audioPath} audio=${audioBytes != null ? "${audioBytes.length}bytes" : "none"}',
    );

    final payload = CortanaReplyPayload(
      text: broadcastText,
      audioPath: audioPath,
      audioBytes: audioBytes,
      audioFormat: audioFormat,
      actionPlan: actionPlan,
    );

    if (audioPath.isEmpty && audioBytes == null) {
      debugPrint(
        '[Cortana Broadcast] audio missing, fallback to text-only broadcast',
      );
    }

    if (!mounted) return;

    _appendCortanaBroadcastChatMessage(envelope, meta, broadcastText);
    if (!_appInForeground) {
      _pendingBackgroundCortanaBroadcast = payload;
      _markCortanaBroadcastAvailable();
      return;
    }

    _presentCortanaFloatingBroadcast(payload);
  }

  void _appendCortanaBroadcastChatMessage(
    PushEnvelope envelope,
    Map<String, dynamic> meta,
    String broadcastText,
  ) {
    final groupId = (meta['group_id'] ?? '').toString().trim();
    final fromUser = (meta['from_user'] ?? meta['origin'] ?? 'cortana-agent')
        .toString()
        .trim();
    final messageMeta = Map<String, dynamic>.from(meta);
    messageMeta['cortana_broadcast'] = true;
    if (envelope.messageId.isNotEmpty) {
      messageMeta['_message_id'] = envelope.messageId;
    }
    final scopeKey = groupId.isEmpty ? 'direct' : _groupScopeKey(groupId);
    final timestamp = DateTime.fromMillisecondsSinceEpoch(envelope.timestamp);
    final chatMessage = ChatMessage(
      content: broadcastText,
      direction: MessageDirection.incoming,
      timestamp: timestamp,
      scopeKey: scopeKey,
      authorId: fromUser,
      groupId: groupId,
      messageType: 'text',
      meta: messageMeta,
    );

    if (envelope.messageId.isNotEmpty &&
        _replaceMessageById(
          scopeKey: scopeKey,
          messageId: envelope.messageId,
          message: chatMessage,
          updateStatus: 'Received Cortana broadcast',
        )) {
      _seenMessageIds.add(envelope.messageId);
      return;
    }

    _appendMessage(chatMessage, updateStatus: 'Received Cortana broadcast');
    if (envelope.messageId.isNotEmpty) {
      _seenMessageIds.add(envelope.messageId);
    }
  }

  void _markCortanaBroadcastAvailable() {
    setState(() {
      _cortanaBadge = true;
      if (_rootTab != RootTab.cortana &&
          _cortanaFloatingMode == CortanaDisplayMode.collapsed) {
        _cortanaFloatingMode = CortanaDisplayMode.small;
      }
    });
  }

  void _playPendingBackgroundCortanaBroadcast() {
    final payload = _pendingBackgroundCortanaBroadcast;
    if (payload == null || !mounted) {
      return;
    }
    _pendingBackgroundCortanaBroadcast = null;
    _presentCortanaFloatingBroadcast(payload);
  }

  void _presentCortanaFloatingBroadcast(
    CortanaReplyPayload payload, {
    bool forceAutoPlay = false,
  }) {
    _markCortanaBroadcastAvailable();

    debugPrint(
      '[Cortana Broadcast] raise floating cortana mode=$_cortanaFloatingMode text=${payload.text}',
    );

    if (!_cortanaAutoPlay && !forceAutoPlay) {
      return;
    }

    _cortanaBroadcastQueue.enqueueLatest(payload, (nextPayload, onFinished) {
      _playQueuedCortanaBroadcast(nextPayload, onFinished);
    });
  }

  void _maybePresentCortanaReplyFromIncoming(ChatMessage message) {
    if (message.direction != MessageDirection.incoming) {
      return;
    }
    final payload = _extractCortanaReplyPayload(message);
    if (payload == null) {
      return;
    }
    if (!payload.hasAudio && payload.suggestedReplies.isEmpty) {
      return;
    }
    if (payload.requestId.isNotEmpty &&
        _pendingCortanaRequestIds.contains(payload.requestId)) {
      return;
    }
    final replyKey = _buildCortanaReplyKey(message);
    if (!_presentedCortanaReplyKeys.add(replyKey)) {
      return;
    }
    if (!_appInForeground) {
      _pendingBackgroundCortanaBroadcast = payload;
      _markCortanaBroadcastAvailable();
      return;
    }
    _presentCortanaFloatingBroadcast(payload, forceAutoPlay: true);
  }

  void _playQueuedCortanaBroadcast(
    CortanaReplyPayload payload,
    void Function() onFinished, {
    int retryCount = 0,
  }) {
    final cortanaState = _cortanaPageKey.currentState;
    if (cortanaState == null) {
      if (retryCount >= 10) {
        debugPrint(
          '[Cortana Broadcast] CortanaPage not ready after retries, drop broadcast: ${payload.text}',
        );
        onFinished();
        return;
      }
      WidgetsBinding.instance.addPostFrameCallback((_) {
        Future<void>.delayed(const Duration(milliseconds: 120), () {
          if (!mounted) {
            onFinished();
            return;
          }
          _playQueuedCortanaBroadcast(
            payload,
            onFinished,
            retryCount: retryCount + 1,
          );
        });
      });
      return;
    }

    cortanaState.playBroadcast(
      payload,
      onFinished: () {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) {
            return;
          }
          final hasPending = _cortanaBroadcastQueue.hasPending;
          if (_cortanaBadge == hasPending) {
            return;
          }
          setState(() {
            _cortanaBadge = hasPending;
          });
        });
        onFinished();
      },
    );
  }

  Future<CortanaReplyPayload> _sendCortanaMessage(String message) async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      throw Exception('Please login first');
    }

    final startTime = DateTime.now();
    const replyTimeout = Duration(seconds: 20);
    final requestId = _buildCortanaRequestId();
    _pendingCortanaRequestIds.add(requestId);
    final deviceContext =
        await _refreshCortanaDeviceContext(report: true) ??
        _currentCortanaDeviceContext();
    addFlutterClientLog('Cortana 发送: $message');
    final meta = <String, dynamic>{
      'conversation_mode': 'cortana',
      'input_mode': 'cortana_text',
      'reply_mode': 'audio_preferred',
      'cortana_request_id': requestId,
      'device_context': ?deviceContext,
    };
    _appendOutgoing(
      message,
      meta: meta,
      scopeKeyOverride: 'direct',
      groupIdOverride: '',
    );
    final baselineCount =
        (_historyByScope['direct'] ?? const <ChatMessage>[]).length;

    try {
      await _runAuthed('Send Cortana message', (client) {
        return client.sendAppMessage(message, meta: meta);
      });

      while (DateTime.now().difference(startTime) < replyTimeout) {
        await Future.delayed(const Duration(milliseconds: 500));
        final directMessages =
            _historyByScope['direct'] ?? const <ChatMessage>[];
        final appendedMessages = directMessages.skip(baselineCount);
        for (final msg in appendedMessages.toList().reversed) {
          final replyKey = _buildCortanaReplyKey(msg);
          if (_consumedCortanaReplyKeys.contains(replyKey)) {
            continue;
          }
          final payload = _extractCortanaReplyPayload(msg);
          if (payload == null) {
            continue;
          }
          if (payload.requestId.isNotEmpty && payload.requestId != requestId) {
            continue;
          }
          _consumedCortanaReplyKeys.add(replyKey);
          addFlutterClientLog(
            'Cortana 回复: ${payload.text.length > 80 ? '${payload.text.substring(0, 80)}...' : payload.text}',
          );
          return payload;
        }
      }

      throw Exception('No response from assistant');
    } catch (err) {
      addFlutterClientLog('Cortana 失败: $err');
      throw Exception('Failed to send message: $err');
    } finally {
      _pendingCortanaRequestIds.remove(requestId);
    }
  }

  Future<List<CortanaReplayItem>> _loadCortanaVoiceHistory() {
    return _runAuthed('Load Cortana history', (client) {
      return client.listCortanaVoiceHistory();
    });
  }

  Future<CortanaReplayItem> _prepareCortanaVoicePlayback(
    CortanaReplayItem item,
  ) async {
    if (item.audioPath.trim().isNotEmpty &&
        await File(item.audioPath).exists()) {
      return item;
    }
    if (item.audioBytes != null) {
      return item;
    }
    if (item.fileId.trim().isEmpty) {
      return item;
    }

    final extension = item.audioFormat.trim().isEmpty
        ? 'bin'
        : item.audioFormat;
    final audioPath = await _attachmentPathForFileID(
      fileId: item.fileId,
      subdir: 'cortana_broadcasts',
      prefix: 'cortana',
      extension: extension,
    );
    final audioFile = File(audioPath);
    if (!await audioFile.exists()) {
      await _runAuthed('Download Cortana history audio', (client) {
        return client.downloadAttachmentToFile(
          item.fileId,
          destinationPath: audioPath,
          attachmentMeta: <String, dynamic>{
            'file_id': item.fileId,
            'audio_format': item.audioFormat,
            'storage_provider': item.storageProvider,
            'object_key': item.objectKey,
          },
          onProgress: (receivedBytes, totalBytes, resumed) {
            _updateDownloadStatus(
              label: 'Cortana 播报',
              receivedBytes: receivedBytes,
              totalBytes: totalBytes,
              resumed: resumed,
            );
          },
        );
      });
      _clearDownloadStatus(successText: 'Cortana 播报下载完成');
    }

    return CortanaReplayItem(
      id: item.id,
      text: item.text,
      audioPath: audioPath,
      audioBytes: item.audioBytes,
      audioFormat: item.audioFormat,
      createdAt: item.createdAt,
      actionPlan: item.actionPlan,
      sourceLabel: item.sourceLabel,
      fileId: item.fileId,
      storageProvider: item.storageProvider,
      objectKey: item.objectKey,
    );
  }

  Future<void> _openCortanaHistoryPage() async {
    final personaName = _cortanaSettings.personaName.trim().isEmpty
        ? 'Cortana'
        : _cortanaSettings.personaName.trim();
    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => CortanaHistoryPage(
          onLoadHistory: _loadCortanaVoiceHistory,
          onPreparePlayback: _prepareCortanaVoicePlayback,
          title: '$personaName 播报历史',
        ),
      ),
    );
  }

  void _reportCortanaUiEvent(String eventKind, {String summary = ''}) {
    final deviceContext = _currentCortanaDeviceContext();
    unawaited(
      _runAuthed('Report Cortana UI event', (client) {
        return client.sendCortanaEvent(
          eventKind,
          meta: <String, dynamic>{
            'summary': summary,
            'root_tab': _rootTab.name,
            'floating_mode': _cortanaFloatingMode.name,
            'device_context': ?deviceContext,
          },
        );
      }).catchError((Object err, StackTrace _) {
        debugPrint('[Cortana UI Event] $eventKind failed: $err');
      }),
    );
  }

  Future<T> _runAuthed<T>(
    String operation,
    Future<T> Function(AppAgentClient client) action,
  ) async {
    final ready = await _ensureSessionReady();
    if (!ready) {
      throw const AppAgentUnauthorizedException('Please login first.');
    }
    try {
      return await action(_client);
    } catch (err) {
      if (!_isUnauthorizedError(err)) {
        rethrow;
      }
      final refreshed = await _ensureSessionReady(forceRefresh: true);
      if (!refreshed) {
        throw const AppAgentUnauthorizedException('Please login first.');
      }
      return action(_client);
    }
  }

  Future<void> _logout() async {
    final refreshToken = _refreshToken.trim();
    try {
      if (_clientConfig != null &&
          (_sessionToken.isNotEmpty || refreshToken.isNotEmpty)) {
        await _client.logout(refreshToken: refreshToken);
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Logout'));
    } finally {
      await _clearLocalAuthState(status: 'Logged out');
      _appendSystem('已清除本地登录凭据。');
    }
  }

  AppAgentClient get _client => AppAgentClient(
    baseUrl: _clientConfig?.baseUrl ?? '',
    userId: _userIdController.text.trim(),
    password: _passwordController.text,
    receiveToken: _clientConfig?.receiveToken ?? '',
    sessionToken: _sessionToken,
    obsAgentBaseUrl: _obsAgentBaseUrl,
  );
}
