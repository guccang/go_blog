// ignore_for_file: invalid_use_of_protected_member
part of '../../main.dart';

extension _ChatPageStateCodegen on _ChatPageState {
  CodingProjectInfo? get _selectedCodingProject {
    for (final project in _codingProjects) {
      if (project.qualifiedName == _selectedCodeProjectQualifiedName) {
        return project;
      }
    }
    return null;
  }

  List<String> get _selectedCodingProjectTools {
    final project = _selectedCodingProject;
    if (project == null) {
      return const <String>[];
    }
    if (project.availableTools.isEmpty) {
      if (project.defaultTool.isNotEmpty) {
        return <String>[project.defaultTool];
      }
      return const <String>['claudecode'];
    }
    return List<String>.from(project.availableTools);
  }

  List<String> get _selectedToolSettingsOptions {
    final project = _selectedCodingProject;
    if (project == null) {
      return const <String>[];
    }
    if (_selectedCodeTool == 'codex') {
      return List<String>.from(project.codexSettings);
    }
    return List<String>.from(project.claudeCodeSettings);
  }

  bool get _selectedCodeToolSupportsResume =>
      codegenResumeIdentifierForTool(_selectedCodeTool).isNotEmpty;

  DeployProjectInfo? get _selectedDeployProject {
    for (final project in _deployProjects) {
      if (project.qualifiedName == _selectedDeployProjectQualifiedName) {
        return project;
      }
    }
    return null;
  }

  List<String> _buildProjectSearchFields(
    String name,
    String agent,
    String qualifiedName,
  ) {
    return <String>[name, agent, qualifiedName]
        .map(_normalizeProjectSearchText)
        .where((item) => item.isNotEmpty)
        .toList();
  }

  String _normalizeProjectSearchText(String input) {
    return input.trim().toLowerCase().replaceAll(RegExp(r'[\s_\-@./\\]+'), '');
  }

  bool _isSubsequenceMatch(String needle, String haystack) {
    if (needle.isEmpty) {
      return true;
    }
    var haystackIndex = 0;
    for (final codeUnit in needle.codeUnits) {
      var found = false;
      while (haystackIndex < haystack.length) {
        if (haystack.codeUnitAt(haystackIndex) == codeUnit) {
          haystackIndex++;
          found = true;
          break;
        }
        haystackIndex++;
      }
      if (!found) {
        return false;
      }
    }
    return true;
  }

  bool _matchesProjectSearch(
    String query,
    String name,
    String agent,
    String qualifiedName,
  ) {
    final tokens = query
        .split(RegExp(r'\s+'))
        .map(_normalizeProjectSearchText)
        .where((item) => item.isNotEmpty)
        .toList();
    if (tokens.isEmpty) {
      return true;
    }
    final fields = _buildProjectSearchFields(name, agent, qualifiedName);
    return tokens.every(
      (token) => fields.any(
        (field) => field.contains(token) || _isSubsequenceMatch(token, field),
      ),
    );
  }

  List<CodingProjectInfo> get _filteredCodingProjects {
    final query = _codegenCodeSearchController.text.trim().toLowerCase();
    if (query.isEmpty) {
      return List<CodingProjectInfo>.from(_codingProjects);
    }
    return _codingProjects.where((project) {
      return _matchesProjectSearch(
        query,
        project.name,
        project.agent,
        project.qualifiedName,
      );
    }).toList();
  }

  List<DeployProjectInfo> get _filteredDeployProjects {
    final query = _codegenDeploySearchController.text.trim().toLowerCase();
    if (query.isEmpty) {
      return List<DeployProjectInfo>.from(_deployProjects);
    }
    return _deployProjects.where((project) {
      return _matchesProjectSearch(
        query,
        project.name,
        project.agent,
        project.qualifiedName,
      );
    }).toList();
  }

  void _syncFilteredCodegenSelections() {
    final filteredCoding = _filteredCodingProjects;
    if (filteredCoding.isNotEmpty &&
        filteredCoding.every(
          (project) =>
              project.qualifiedName != _selectedCodeProjectQualifiedName,
        )) {
      _selectedCodeProjectQualifiedName = filteredCoding.first.qualifiedName;
    }

    final filteredDeploy = _filteredDeployProjects;
    if (filteredDeploy.isNotEmpty &&
        filteredDeploy.every(
          (project) =>
              project.qualifiedName != _selectedDeployProjectQualifiedName,
        )) {
      _selectedDeployProjectQualifiedName = filteredDeploy.first.qualifiedName;
    }

    _syncCodegenSelections();
  }

  Future<void> _restoreCodegenPreferences() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final modeName = prefs.getString(_codegenModeKey)?.trim() ?? '';
      final codeProject = prefs.getString(_codeProjectKey)?.trim() ?? '';
      final codeTool = prefs.getString(_codeToolKey)?.trim() ?? '';
      final claudeSettings = prefs.getString(_claudeSettingsKey)?.trim() ?? '';
      final deployProject = prefs.getString(_deployProjectKey)?.trim() ?? '';
      final deployTarget = prefs.getString(_deployTargetKey)?.trim() ?? '';
      final deployArgs = prefs.getString(_deployArgsKey)?.trim() ?? '';
      final codeSearch = prefs.getString(_codeSearchKey)?.trim() ?? '';
      final deploySearch = prefs.getString(_deploySearchKey)?.trim() ?? '';
      final debugBundleMode = prefs.getBool(_debugBundleModeKey) ?? false;
      final resumeLastSession = prefs.getBool(_resumeLastSessionKey) ?? false;
      if (!mounted) {
        _deployArgsController.text = deployArgs;
        _codegenCodeSearchController.text = codeSearch;
        _codegenDeploySearchController.text = deploySearch;
        _selectedCodeProjectQualifiedName = codeProject;
        _selectedCodeTool = codeTool;
        _selectedClaudeSettings = claudeSettings;
        _selectedDeployProjectQualifiedName = deployProject;
        _selectedDeployTarget = deployTarget;
        _codegenDebugBundleMode = debugBundleMode;
        _codegenResumeLastSession = resumeLastSession;
        if (_codegenDebugBundleMode) {
          _codegenAutoDeploy = false;
        }
        _codegenMode = modeName == CodegenLaunchMode.deploy.name
            ? CodegenLaunchMode.deploy
            : CodegenLaunchMode.code;
        return;
      }
      setState(() {
        _deployArgsController.text = deployArgs;
        _codegenCodeSearchController.text = codeSearch;
        _codegenDeploySearchController.text = deploySearch;
        _selectedCodeProjectQualifiedName = codeProject;
        _selectedCodeTool = codeTool;
        _selectedClaudeSettings = claudeSettings;
        _selectedDeployProjectQualifiedName = deployProject;
        _selectedDeployTarget = deployTarget;
        _codegenDebugBundleMode = debugBundleMode;
        _codegenResumeLastSession = resumeLastSession;
        if (_codegenDebugBundleMode) {
          _codegenAutoDeploy = false;
        }
        _codegenMode = modeName == CodegenLaunchMode.deploy.name
            ? CodegenLaunchMode.deploy
            : CodegenLaunchMode.code;
      });
    } catch (_) {
      // Ignore local preference restore failures.
    }
  }

  Future<void> _persistCodegenPreferences() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_codegenModeKey, _codegenMode.name);
      await prefs.setString(_codeProjectKey, _selectedCodeProjectQualifiedName);
      await prefs.setString(_codeToolKey, _selectedCodeTool);
      await prefs.setString(_claudeSettingsKey, _selectedClaudeSettings);
      await prefs.setBool(_resumeLastSessionKey, _codegenResumeLastSession);
      await prefs.setString(
        _deployProjectKey,
        _selectedDeployProjectQualifiedName,
      );
      await prefs.setString(_deployTargetKey, _selectedDeployTarget);
      await prefs.setString(_deployArgsKey, _deployArgsController.text.trim());
      await prefs.setString(
        _codeSearchKey,
        _codegenCodeSearchController.text.trim(),
      );
      await prefs.setString(
        _deploySearchKey,
        _codegenDeploySearchController.text.trim(),
      );
      await prefs.setBool(_debugBundleModeKey, _codegenDebugBundleMode);

      // 保存历史记录
      final historyJson = jsonEncode(
        _codegenHistory.map((item) => item.toJson()).toList(),
      );
      await prefs.setString(_codegenHistoryKey, historyJson);
      await _secureStorage.write(
        key: _codegenHistoryBackupKey,
        value: historyJson,
      );
    } catch (_) {
      // Ignore local preference persistence failures.
    }
  }

  Future<void> _loadCodegenHistory() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      var historyJson = prefs.getString(_codegenHistoryKey)?.trim() ?? '';
      if (historyJson.isEmpty) {
        try {
          historyJson =
              (await _secureStorage.read(
                key: _codegenHistoryBackupKey,
              ))?.trim() ??
              '';
        } catch (_) {
          historyJson = '';
        }
      }
      if (historyJson.isNotEmpty) {
        final historyList = jsonDecode(historyJson) as List<dynamic>;
        setState(() {
          _codegenHistory = historyList
              .map(
                (item) => _normalizeCodegenHistoryItem(
                  CodegenHistoryItem.fromJson(item as Map<String, dynamic>),
                ),
              )
              .toList();
        });
        _publishCodegenHistory();
        if ((prefs.getString(_codegenHistoryKey) ?? '').trim().isEmpty) {
          unawaited(_persistCodegenPreferences());
        }
        return;
      }

      final migratedHistory = await _migrateCodegenHistoryFromChat(prefs);
      if (migratedHistory.isNotEmpty) {
        setState(() {
          _codegenHistory = migratedHistory;
        });
        _publishCodegenHistory();
        unawaited(_persistCodegenPreferences());
      }
    } catch (_) {
      // Ignore history load failures.
    }
  }

  Future<List<CodegenHistoryItem>> _migrateCodegenHistoryFromChat(
    SharedPreferences prefs,
  ) async {
    final userId = _userIdController.text.trim();
    final migrated = <CodegenHistoryItem>[];
    for (final key in prefs.getKeys()) {
      final scopeKey = _extractScopeKey(key, userId, _historyStoragePrefix);
      if (scopeKey == null) {
        continue;
      }
      final historyJson = prefs.getString(key)?.trim() ?? '';
      if (historyJson.isEmpty) {
        continue;
      }
      try {
        final messages = jsonDecode(historyJson) as List<dynamic>;
        for (final raw in messages) {
          final message = ChatMessage.fromJson(raw as Map<String, dynamic>);
          final command = message.content.trim();
          if (!command.startsWith('/cg ')) {
            continue;
          }
          final mode = command.startsWith('/cg deploy')
              ? CodegenLaunchMode.deploy
              : command.startsWith('/cg history-backup')
              ? CodegenLaunchMode.backup
              : CodegenLaunchMode.code;
          migrated.add(
            CodegenHistoryItem(
              id: _newCodegenHistoryId(),
              timestamp: message.timestamp,
              command: command,
              mode: mode,
              completed: true,
            ),
          );
        }
      } catch (_) {
        // Ignore one broken chat scope and keep migrating the rest.
      }
    }
    migrated.sort((a, b) => b.timestamp.compareTo(a.timestamp));
    final seen = <String>{};
    return migrated
        .where((item) {
          final key =
              '${item.timestamp.microsecondsSinceEpoch}|${item.command}';
          return seen.add(key);
        })
        .take(1000)
        .toList();
  }

  void _publishCodegenHistory() {
    _codegenHistoryNotifier.value = List<CodegenHistoryItem>.unmodifiable(
      _codegenHistory,
    );
  }

  void _mutateCodegenHistory(VoidCallback mutation) {
    if (!mounted || _rootTab != RootTab.codegen) {
      mutation();
      return;
    }
    setState(mutation);
  }

  int _runningCodegenCount(CodegenLaunchMode mode) => _codegenHistory
      .where((item) => item.mode == mode && !item.completed)
      .length;

  String _newCodegenHistoryId() =>
      'cg_${DateTime.now().microsecondsSinceEpoch.toString()}';

  void _startCodegenTimeoutSweepTimer() {
    _codegenTimeoutSweepTimer?.cancel();
    _codegenTimeoutSweepTimer = Timer.periodic(
      _codegenHistoryTimeoutSweepInterval,
      (_) => _markTimedOutCodegenHistoryItems(),
    );
    _markTimedOutCodegenHistoryItems();
  }

  bool _isCodegenHistoryTimedOut(CodegenHistoryItem item) {
    return !item.completed &&
        DateTime.now().difference(item.timestamp) >= _codegenHistoryTimeout;
  }

  CodegenHistoryItem _markCodegenHistoryItemTimedOut(CodegenHistoryItem item) {
    final hasTimeoutEntry = item.processEntries.any(
      (entry) => entry.origin == 'client-timeout',
    );
    return item.copyWith(
      completed: true,
      processEntries: hasTimeoutEntry
          ? item.processEntries
          : <CodegenProcessEntry>[
              ...item.processEntries,
              CodegenProcessEntry(
                timestamp: DateTime.now(),
                content: '任务超时: 超过 1 小时未收到结束消息，客户端已自动结束统计。',
                origin: 'client-timeout',
              ),
            ],
    );
  }

  void _markTimedOutCodegenHistoryItems() {
    if (!mounted) {
      return;
    }
    if (_codegenHistory.isEmpty) {
      return;
    }
    var changed = false;
    final updated = _codegenHistory
        .map((item) {
          if (!_isCodegenHistoryTimedOut(item)) {
            return item;
          }
          changed = true;
          return _markCodegenHistoryItemTimedOut(item);
        })
        .toList(growable: false);
    if (!changed) {
      return;
    }
    setState(() {
      _codegenHistory = updated;
      if (_activeCodegenHistoryId.isNotEmpty &&
          _codegenHistory.any(
            (item) => item.id == _activeCodegenHistoryId && item.completed,
          )) {
        _activeCodegenHistoryId = '';
      }
    });
    _publishCodegenHistory();
    unawaited(_persistCodegenPreferences());
  }

  CodegenHistoryItem _normalizeCodegenHistoryItem(CodegenHistoryItem item) {
    final completed = item.completed || _codegenHistoryItemLooksCompleted(item);
    if (!completed && _isCodegenHistoryTimedOut(item)) {
      return _markCodegenHistoryItemTimedOut(item);
    }
    if (item.id.trim().isNotEmpty) {
      return completed == item.completed
          ? item
          : item.copyWith(completed: true);
    }
    return CodegenHistoryItem(
      id: _newCodegenHistoryId(),
      timestamp: item.timestamp,
      command: item.command,
      mode: item.mode,
      locked: item.locked,
      completed: completed,
      requestId: item.requestId,
      processEntries: item.processEntries,
    );
  }

  bool _codegenHistoryItemLooksCompleted(CodegenHistoryItem item) {
    for (final entry in item.processEntries) {
      final text = entry.content.trim();
      if (text.startsWith('任务完成:') ||
          text.startsWith('任务取消:') ||
          text.startsWith('强制总结:') ||
          text.startsWith('Codegen task completed') ||
          text.startsWith('Codegen task failed')) {
        return true;
      }
    }
    return false;
  }

  List<String> _splitCodegenProcessContent(
    String content, {
    int firstByteLimit = _codegenProcessEntryByteLimit,
  }) {
    final text = content.trimRight();
    if (text.isEmpty) {
      return const <String>[];
    }
    final initialLimit = firstByteLimit <= 0
        ? _codegenProcessEntryByteLimit
        : firstByteLimit;
    final bytes = utf8.encode(text);
    if (bytes.length <= initialLimit) {
      return <String>[text];
    }

    final chunks = <String>[];
    var usedBytes = 0;
    final buffer = StringBuffer();
    for (final rune in text.runes) {
      final char = String.fromCharCode(rune);
      final charBytes = utf8.encode(char).length;
      final currentLimit = chunks.isEmpty
          ? initialLimit
          : _codegenProcessEntryByteLimit;
      if (usedBytes > 0 && usedBytes + charBytes > currentLimit) {
        chunks.add(buffer.toString());
        buffer.clear();
        usedBytes = 0;
      }
      buffer.write(char);
      usedBytes += charBytes;
    }
    if (buffer.isNotEmpty) {
      chunks.add(buffer.toString());
    }
    return chunks;
  }

  CodegenHistoryItem _addCodegenHistory(
    String command,
    CodegenLaunchMode mode,
  ) {
    final item = CodegenHistoryItem(
      id: _newCodegenHistoryId(),
      timestamp: DateTime.now(),
      command: command,
      mode: mode,
    );
    setState(() {
      _codegenHistory.insert(0, item);
      // 只保留最近1000条记录，但锁定的记录不会被移除
      if (_codegenHistory.length > 1000) {
        final locked = _codegenHistory.where((e) => e.locked).toList();
        final unlocked = _codegenHistory.where((e) => !e.locked).toList();
        if (unlocked.length > 1000 - locked.length) {
          _codegenHistory = [
            ...locked,
            ...unlocked.sublist(0, 1000 - locked.length),
          ];
        }
      }
    });
    _publishCodegenHistory();
    unawaited(_persistCodegenPreferences());
    return item;
  }

  CodegenHistoryItem? _findCodegenHistoryItemById(String itemId) {
    final id = itemId.trim();
    if (id.isEmpty) {
      return null;
    }
    for (final item in _codegenHistory) {
      if (item.id == id) {
        return item;
      }
    }
    return null;
  }

  String? _codegenHistoryIdForMessage(ChatMessage message) {
    return _codegenHistoryItemForProcessMessage(message)?.id;
  }

  CodegenHistoryItem? _codegenHistoryItemForProcessMessage(
    ChatMessage message,
  ) {
    final meta = message.meta ?? const <String, dynamic>{};
    final explicitId = (meta['codegen_history_id'] ?? '').toString().trim();
    final explicitItem = _findCodegenHistoryItemById(explicitId);
    if (explicitItem != null) {
      return explicitItem;
    }

    final requestId = (meta['request_id'] ?? '').toString().trim();
    if (requestId.isNotEmpty) {
      for (final item in _codegenHistory) {
        if (item.requestId == requestId) {
          return item;
        }
      }
    }

    final contentRequestId = extractCodegenRequestIdFromText(message.content);
    if (contentRequestId.isNotEmpty) {
      for (final item in _codegenHistory) {
        if (item.requestId == contentRequestId) {
          return item;
        }
      }
    }

    final sessionId = (meta['session_id'] ?? '').toString().trim();
    if (sessionId.isNotEmpty) {
      for (final item in _codegenHistory) {
        if (item.processEntries.any((entry) => entry.sessionId == sessionId)) {
          return item;
        }
      }
    }
    return null;
  }

  CodegenHistoryItem? _ensureCodegenHistoryForProcessMessage(
    ChatMessage message, {
    String? historyId,
    String? sessionId,
    String? requestId,
    CodegenLaunchMode? mode,
  }) {
    final meta = message.meta ?? const <String, dynamic>{};
    final effectiveHistoryId = (historyId ?? (meta['codegen_history_id'] ?? '').toString()).trim();
    final effectiveSessionId = (sessionId ?? (meta['session_id'] ?? '').toString()).trim();
    final effectiveRequestId = (requestId ?? (meta['request_id'] ?? '').toString()).trim();

    final resolvedMode =
        mode ??
        _codegenHistoryModeForProcessMessage(message) ??
        _codegenHistoryModeForStartMessage(message.content);
    if (resolvedMode == null) {
      return null;
    }

    final existing = _findActiveCodegenHistoryItem(
      historyId: effectiveHistoryId,
      sessionId: effectiveSessionId,
      requestId: effectiveRequestId,
      mode: resolvedMode,
    );
    if (existing != null) {
      return existing;
    }

    final id = effectiveHistoryId.isNotEmpty
        ? effectiveHistoryId
        : _newCodegenHistoryId();
    final command = _fallbackCodegenHistoryCommand(
      message,
      sessionId: effectiveSessionId,
      requestId: effectiveRequestId,
    );
    final item = CodegenHistoryItem(
      id: id,
      timestamp: message.timestamp,
      command: command,
      mode: resolvedMode,
      requestId: effectiveRequestId,
    );
    _mutateCodegenHistory(() {
      _codegenHistory.insert(0, item);
      if (_codegenHistory.length > 1000) {
        final locked = _codegenHistory.where((e) => e.locked).toList();
        final unlocked = _codegenHistory.where((e) => !e.locked).toList();
        if (unlocked.length > 1000 - locked.length) {
          _codegenHistory = [
            ...locked,
            ...unlocked.sublist(0, 1000 - locked.length),
          ];
        }
      }
    });
    _publishCodegenHistory();
    _activeCodegenHistoryId = item.id;
    addFlutterClientLog(
      'CodegenHistory auto_create id=${item.id} mode=${item.mode.name} '
      'session=$effectiveSessionId request=$effectiveRequestId',
    );
    unawaited(_persistCodegenPreferences());
    return item;
  }

  String _fallbackCodegenHistoryCommand(
    ChatMessage message, {
    required String sessionId,
    required String requestId,
  }) {
    final content = message.content.trim();
    if (looksLikeCodegenStartMessage(content)) {
      return content;
    }
    final parts = <String>['远端编码任务'];
    final normalizedRequestId = requestId.trim();
    final normalizedSessionId = sessionId.trim();
    if (normalizedRequestId.isNotEmpty) {
      parts.add('请求: $normalizedRequestId');
    }
    if (normalizedSessionId.isNotEmpty) {
      parts.add('会话: $normalizedSessionId');
    }
    return parts.join('\n');
  }

  void _applyCodegenHistoryItem(CodegenHistoryItem item) {
    final details = CodegenHistoryCommandDetails.parse(item);
    if (details.mode == CodegenLaunchMode.backup) {
      _appendSystem('备份记录不需要回填表单，可以直接重新执行备份。');
      return;
    }
    setState(() {
      _codegenMode = details.mode;
      if (details.mode == CodegenLaunchMode.code) {
        if (details.projectQualifiedName.isNotEmpty &&
            _codingProjects.any(
              (project) =>
                  project.qualifiedName == details.projectQualifiedName,
            )) {
          _selectedCodeProjectQualifiedName = details.projectQualifiedName;
        }
        _selectedCodeTool = details.tool;
        _selectedClaudeSettings = details.claudeSettings;
        _codegenResumeLastSession = details.resumeLastSession;
        _codegenAutoDeploy = details.autoDeploy;
        _codegenPromptController.text = details.requestText;
      } else {
        if (details.projectQualifiedName.isNotEmpty &&
            _deployProjects.any(
              (project) =>
                  project.qualifiedName == details.projectQualifiedName,
            )) {
          _selectedDeployProjectQualifiedName = details.projectQualifiedName;
        }
        _syncCodegenSelections();
        if (details.target.isNotEmpty &&
            _selectedDeployProject?.deployTargets.contains(details.target) ==
                true) {
          _selectedDeployTarget = details.target;
        }
        _deployPackOnly = details.packOnly;
        _deployArgsController.text = details.extraArgs;
      }
    });
    unawaited(_persistCodegenPreferences());
  }

  void _reExecuteCodegenHistory(CodegenHistoryItem item) {
    if (item.mode == CodegenLaunchMode.backup) {
      final details = CodegenHistoryCommandDetails.parse(item);
      final backupType = details.requestText.trim() == 'full'
          ? CodegenHistoryBackupType.full
          : CodegenHistoryBackupType.incremental;
      unawaited(_backupCodegenHistory(backupType));
      return;
    }
    final historyItem = _addCodegenHistory(item.command, item.mode);
    final action = _buildCodegenActionFromDetails(
      CodegenHistoryCommandDetails.parse(item),
      historyItem.id,
      sourceCommand: item.command,
    );
    setState(() {
      _codegenSending = true;
    });
    _runAuthed('Re-execute codegen command', (client) {
          return client.submitCodegenAction(action);
        })
        .then((_) {
          if (mounted) {
            setState(() {
              _status = item.mode == CodegenLaunchMode.code
                  ? 'Code command sent'
                  : 'Deploy command sent';
            });
          }
          _triggerCortanaContextualExpression('surprised');
        })
        .catchError((err) {
          _markCodegenHistoryFailed(historyItem.id, err);
          _appendSystem(
            _describeRequestError(err, operation: 'Re-execute codegen command'),
          );
        })
        .whenComplete(() {
          if (mounted) {
            setState(() {
              _codegenSending = false;
            });
          }
        });
  }

  Future<void> _backupCodegenHistory(
    CodegenHistoryBackupType backupType,
  ) async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    final lastBackupAt = prefs.getInt(_codegenHistoryLastBackupAtKey) ?? 0;
    final historyForBackup = backupType == CodegenHistoryBackupType.full
        ? List<CodegenHistoryItem>.from(_codegenHistory)
        : _codegenHistory
              .where(
                (item) =>
                    item.mode != CodegenLaunchMode.backup &&
                    item.timestamp.millisecondsSinceEpoch > lastBackupAt,
              )
              .toList();
    if (backupType == CodegenHistoryBackupType.incremental &&
        historyForBackup.isEmpty) {
      _appendSystem('没有需要增量备份的新命令记录。');
      return;
    }

    final command =
        '/cg history-backup --type ${backupType.name} '
        '--count ${historyForBackup.length}';
    final item = _addCodegenHistory(command, CodegenLaunchMode.backup);
    setState(() {
      _codegenSending = true;
    });
    try {
      final result = await _runAuthed('Backup codegen history', (client) {
        return client.backupCodegenHistory(
          backupType: backupType,
          history: historyForBackup,
        );
      });
      await prefs.setInt(
        _codegenHistoryLastBackupAtKey,
        DateTime.now().millisecondsSinceEpoch,
      );
      final objectKey = (result['object_key'] ?? '').toString().trim();
      final storageProvider = (result['storage_provider'] ?? '').toString();
      _appendSystem(
        '${backupType.label}完成：${historyForBackup.length} 条，'
        '存储=${storageProvider.isEmpty ? 'local' : storageProvider}'
        '${objectKey.isEmpty ? '' : '，OBS=$objectKey'}',
      );
      setState(() {
        final idx = _codegenHistory.indexWhere((entry) => entry.id == item.id);
        if (idx != -1) {
          _codegenHistory[idx] = _codegenHistory[idx].copyWith(completed: true);
        }
        _status = 'Codegen history backed up';
      });
      _publishCodegenHistory();
      unawaited(_persistCodegenPreferences());
    } catch (err) {
      _appendSystem(
        _describeRequestError(err, operation: 'Backup codegen history'),
      );
    } finally {
      if (mounted) {
        setState(() {
          _codegenSending = false;
        });
      }
    }
  }

  Future<void> _loadCodegenHistoryBackupFromObs() async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    setState(() {
      _codegenSending = true;
    });
    try {
      final backups = await _runAuthed(
        'List codegen history backups',
        (client) => client.listCodegenHistoryBackups(),
      );
      if (backups.isEmpty) {
        _appendSystem('OBS 中没有可加载的编码发布历史备份。');
        return;
      }
      backups.sort((a, b) => b.createdAt.compareTo(a.createdAt));
      List<CodegenHistoryItem> restored = const <CodegenHistoryItem>[];
      CodegenHistoryBackupItem? selected;
      Object? lastLoadError;
      for (final backup in backups) {
        try {
          restored = await _runAuthed(
            'Load codegen history backup',
            (client) => client.loadCodegenHistoryBackup(backup.objectKey),
          );
          selected = backup;
          break;
        } catch (err) {
          lastLoadError = err;
        }
      }
      if (selected == null) {
        throw lastLoadError ?? StateError('No codegen history backup loaded');
      }
      if (restored.isEmpty) {
        _appendSystem('备份文件没有可恢复的编码发布记录。');
        return;
      }

      var added = 0;
      final byID = <String, CodegenHistoryItem>{
        for (final item in _codegenHistory) item.id: item,
      };
      for (final item in restored) {
        if (!byID.containsKey(item.id)) {
          byID[item.id] = item;
          added++;
        }
      }
      final merged = byID.values.toList()
        ..sort((a, b) => b.timestamp.compareTo(a.timestamp));
      setState(() {
        _codegenHistory = merged.length > 1000
            ? merged.take(1000).toList()
            : merged;
      });
      _publishCodegenHistory();
      unawaited(_persistCodegenPreferences());
      _appendSystem(
        '已从 OBS 加载编码发布备份：新增 $added 条，当前共 ${_codegenHistory.length} 条。',
      );
    } catch (err) {
      _appendSystem(
        _describeRequestError(err, operation: 'Load codegen history backup'),
      );
    } finally {
      if (mounted) {
        setState(() {
          _codegenSending = false;
        });
      }
    }
  }

  void _toggleCodegenHistoryLock(CodegenHistoryItem item) {
    setState(() {
      final idx = _codegenHistory.indexOf(item);
      if (idx == -1) return;
      _codegenHistory[idx] = item.copyWith(locked: !item.locked);
    });
    _publishCodegenHistory();
    unawaited(_persistCodegenPreferences());
  }

  void _markCodegenHistoryFailed(String itemId, Object err) {
    final idx = _codegenHistory.indexWhere((item) => item.id == itemId);
    if (idx == -1) {
      return;
    }
    final item = _codegenHistory[idx];
    final entries = List<CodegenProcessEntry>.from(item.processEntries)
      ..add(
        CodegenProcessEntry(
          timestamp: DateTime.now(),
          content: _describeRequestError(err, operation: 'Codegen task'),
          origin: 'client-error',
        ),
      );
    setState(() {
      _codegenHistory[idx] = item.copyWith(
        completed: true,
        processEntries: entries,
      );
    });
    if (_activeCodegenHistoryId == itemId) {
      _activeCodegenHistoryId = '';
    }
    _publishCodegenHistory();
    unawaited(_persistCodegenPreferences());
  }

  CodegenHistoryItem? _findActiveCodegenHistoryItem({
    String historyId = '',
    String sessionId = '',
    String requestId = '',
    CodegenLaunchMode? mode,
  }) {
    final normalizedHistoryId = historyId.trim();
    final normalizedSessionId = sessionId.trim();
    final normalizedRequestId = requestId.trim();
    if (normalizedHistoryId.isNotEmpty) {
      for (final item in _codegenHistory) {
        if (!item.completed &&
            (mode == null || item.mode == mode) &&
            item.id == normalizedHistoryId) {
          return item;
        }
      }
    }
    if (normalizedRequestId.isNotEmpty) {
      for (final item in _codegenHistory) {
        if (!item.completed &&
            (mode == null || item.mode == mode) &&
            item.requestId == normalizedRequestId) {
          return item;
        }
      }
    }
    if (normalizedSessionId.isNotEmpty) {
      for (final item in _codegenHistory) {
        if (!item.completed &&
            (mode == null || item.mode == mode) &&
            item.processEntries.any(
              (entry) => entry.sessionId == normalizedSessionId,
            )) {
          return item;
        }
      }
    }

    final pending = _codegenHistory.where((item) {
      if (item.completed) {
        return false;
      }
      if (mode != null && item.mode != mode) {
        return false;
      }
      return !item.processEntries.any(
        (entry) => entry.sessionId.trim().isNotEmpty,
      );
    }).toList();
    if (pending.isNotEmpty) {
      return pending.last;
    }

    for (final item in _codegenHistory.reversed) {
      if (!item.completed && (mode == null || item.mode == mode)) {
        return item;
      }
    }
    return null;
  }

  CodegenLaunchMode? _codegenHistoryModeForProcessMessage(ChatMessage message) {
    final meta = message.meta ?? const <String, dynamic>{};
    final historyItem = _codegenHistoryItemForProcessMessage(message);
    if (historyItem != null) {
      return historyItem.mode;
    }
    final processKind = (meta['process_kind'] ?? '').toString().trim();
    if (processKind == 'deploy') {
      return CodegenLaunchMode.deploy;
    }
    if (processKind == 'codegen') {
      return CodegenLaunchMode.code;
    }
    final origin = (meta['origin'] ?? '').toString().trim();
    if (origin == 'codegen-stream') {
      final text = message.content.trim().toLowerCase();
      if (message.content.contains('部署') ||
          message.content.contains('发布') ||
          text.contains('deploy') ||
          text.contains('release')) {
        return CodegenLaunchMode.deploy;
      }
      return CodegenLaunchMode.code;
    }
    final text = message.content.trim().toLowerCase();
    if (message.content.contains('部署') ||
        message.content.contains('发布') ||
        text.contains('deploy') ||
        text.contains('release')) {
      return CodegenLaunchMode.deploy;
    }
    if (message.content.contains('编码') ||
        message.content.contains('代码') ||
        text.contains('codegen') ||
        text.contains('/cg') ||
        text.contains('claude')) {
      return CodegenLaunchMode.code;
    }
    if (text.startsWith('codegen task completed') ||
        text.startsWith('codegen task failed')) {
      return CodegenLaunchMode.code;
    }
    return null;
  }

  bool _hasPendingCodegenHistoryExecution({CodegenLaunchMode? mode}) =>
      _findActiveCodegenHistoryItem(mode: mode) != null;

  bool _isCodegenProcessMessage(ChatMessage message) {
    final meta = message.meta ?? const <String, dynamic>{};
    final origin = (meta['origin'] ?? '').toString().trim();
    final processKind = (meta['process_kind'] ?? '').toString().trim();
    return meta['app_process'] == true ||
        origin == 'app-process' ||
        origin == 'codegen-stream' ||
        processKind == 'codegen' ||
        processKind == 'deploy';
  }

  void _bindCodegenHistoryRequestFromMessage(ChatMessage message) {
    final content = message.content.trim();
    final requestId = extractCodegenRequestIdFromText(content);
    if (requestId.isEmpty) {
      return;
    }
    final mode = _codegenHistoryModeForStartMessage(content);
    if (mode == null) {
      return;
    }
    final idx = _codegenHistory.lastIndexWhere(
      (item) =>
          !item.completed && item.mode == mode && item.requestId.trim().isEmpty,
    );
    if (idx == -1) {
      _ensureCodegenHistoryForProcessMessage(
        message,
        requestId: requestId,
        mode: mode,
      );
      return;
    }
    _mutateCodegenHistory(() {
      _codegenHistory[idx] = _codegenHistory[idx].copyWith(
        requestId: requestId,
      );
    });
    _publishCodegenHistory();
    unawaited(_persistCodegenPreferences());
  }

  CodegenLaunchMode? _codegenHistoryModeForStartMessage(String content) {
    if (content.contains('编码会话已启动') || content.contains('ACP Debug 会话已启动')) {
      return CodegenLaunchMode.code;
    }
    if (content.contains('发布已启动') ||
        content.contains('部署已启动') ||
        content.contains('自动部署已启动')) {
      return CodegenLaunchMode.deploy;
    }
    return null;
  }

  bool _isTerminalCodegenProcessMessage(ChatMessage message) {
    final text = message.content.trim();
    final meta = message.meta ?? const <String, dynamic>{};
    if (meta['final'] == true) {
      return true;
    }
    if ((meta['app_process'] == true ||
            (meta['origin'] ?? '').toString() == 'app-process') &&
        (text.startsWith('任务完成:') ||
            text.startsWith('任务取消:') ||
            text.startsWith('强制总结:') ||
            text.startsWith('Codegen task completed') ||
            text.startsWith('Codegen task failed'))) {
      return true;
    }
    return false;
  }

  void _appendProcessEntryToActiveCodegenHistory(
    ChatMessage message,
    String content, {
    required String origin,
    String historyId = '',
    String sessionId = '',
    String requestId = '',
    CodegenLaunchMode? mode,
  }) {
    final chunks = _splitCodegenProcessContent(content);
    if (chunks.isEmpty) {
      return;
    }
    final activeItem =
        _findActiveCodegenHistoryItem(
          historyId: historyId,
          sessionId: sessionId,
          requestId: requestId,
          mode: mode,
        ) ??
        _ensureCodegenHistoryForProcessMessage(
          message,
          historyId: historyId,
          sessionId: sessionId,
          requestId: requestId,
          mode: mode,
        );
    if (activeItem == null) {
      addFlutterClientLog(
        'CodegenHistory drop_no_active_item origin=$origin '
        'session=${sessionId.trim()} request=${requestId.trim()} history=${historyId.trim()}',
      );
      return;
    }
    final entries = List<CodegenProcessEntry>.from(activeItem.processEntries);
    final firstChunk = chunks.first;
    final last = entries.isNotEmpty ? entries.last : null;
    if (chunks.length == 1 &&
        last != null &&
        last.content == firstChunk &&
        last.origin == origin &&
        last.sessionId == sessionId) {
      return;
    }
    for (final chunk in chunks) {
      final currentLast = entries.isNotEmpty ? entries.last : null;
      // 流式消息合并：同一个 session 的短增量合并到同一条目，
      // 但每条过程记录仍保持在 2048 UTF-8 bytes 以内。
      if (currentLast != null &&
          origin == 'codegen-stream' &&
          currentLast.origin == 'codegen-stream' &&
          currentLast.sessionId == sessionId) {
        final currentBytes = utf8.encode(currentLast.content).length;
        final remainingBytes = _codegenProcessEntryByteLimit - currentBytes;
        if (remainingBytes <= 0) {
          entries.add(
            CodegenProcessEntry(
              timestamp: message.timestamp,
              content: chunk,
              origin: origin,
              sessionId: sessionId,
            ),
          );
          continue;
        }
        final streamChunks = _splitCodegenProcessContent(
          chunk,
          firstByteLimit: remainingBytes,
        );
        if (streamChunks.isEmpty) {
          continue;
        }
        entries[entries.length - 1] = CodegenProcessEntry(
          timestamp: message.timestamp,
          content: currentLast.content + streamChunks.first,
          origin: origin,
          sessionId: sessionId,
        );
        for (final streamChunk in streamChunks.skip(1)) {
          entries.add(
            CodegenProcessEntry(
              timestamp: message.timestamp,
              content: streamChunk,
              origin: origin,
              sessionId: sessionId,
            ),
          );
        }
        continue;
      }
      entries.add(
        CodegenProcessEntry(
          timestamp: message.timestamp,
          content: chunk,
          origin: origin,
          sessionId: sessionId,
        ),
      );
    }
    final updatedItem = activeItem.copyWith(
      processEntries: entries,
      requestId: activeItem.requestId.isEmpty ? requestId.trim() : null,
      completed: _isTerminalCodegenProcessMessage(message),
    );
    _mutateCodegenHistory(() {
      final idx = _codegenHistory.indexWhere(
        (item) => item.id == activeItem.id,
      );
      if (idx == -1) {
        return;
      }
      _codegenHistory[idx] = updatedItem;
    });
    _publishCodegenHistory();
    addFlutterClientLog(
      'CodegenHistory append id=${updatedItem.id} origin=$origin '
      'entries=${updatedItem.processEntries.length} completed=${updatedItem.completed} '
      'session=${sessionId.trim()} request=${requestId.trim()}',
    );
    if (updatedItem.completed) {
      _activeCodegenHistoryId = '';
    } else {
      _activeCodegenHistoryId = updatedItem.id;
    }
    unawaited(_persistCodegenPreferences());
  }

  bool _routeIncomingCodegenProcessMessage(ChatMessage message) {
    final meta = message.meta ?? const <String, dynamic>{};
    var origin = (meta['origin'] ?? '').toString().trim();
    if (!_isCodegenProcessMessage(message)) {
      return false;
    }
    if (origin.isEmpty) {
      origin = (meta['process_kind'] ?? '').toString().trim().isEmpty
          ? 'app-process'
          : 'app-process';
    }
    final mode = _codegenHistoryModeForProcessMessage(message);
    final hasActiveTask = _hasPendingCodegenHistoryExecution(mode: mode);
    if (!hasActiveTask) {
      final created = _ensureCodegenHistoryForProcessMessage(
        message,
        mode: mode,
      );
      if (created == null) {
        return false;
      }
    }
    final sessionId = (meta['session_id'] ?? '').toString().trim();
    final requestId = (meta['request_id'] ?? '').toString().trim();
    final historyId = (meta['codegen_history_id'] ?? '').toString().trim();
    _appendProcessEntryToActiveCodegenHistory(
      message,
      message.content,
      origin: origin,
      historyId: historyId,
      sessionId: sessionId,
      requestId: requestId,
      mode: mode,
    );
    return true;
  }

  void _completeCodegenHistoryForStream(_CodegenStreamState state) {
    final sessionId = (state.latestMessage.meta?['session_id'] ?? '')
        .toString()
        .trim();
    final activeItem = _findActiveCodegenHistoryItem(
      historyId: (state.latestMessage.meta?['codegen_history_id'] ?? '')
          .toString(),
      sessionId: sessionId,
      requestId: (state.latestMessage.meta?['request_id'] ?? '').toString(),
      mode: _codegenHistoryModeForProcessMessage(state.latestMessage),
    );
    if (activeItem == null || activeItem.completed) {
      return;
    }
    _mutateCodegenHistory(() {
      final idx = _codegenHistory.indexWhere(
        (item) => item.id == activeItem.id,
      );
      if (idx == -1) {
        return;
      }
      _codegenHistory[idx] = activeItem.copyWith(completed: true);
    });
    if (_activeCodegenHistoryId == activeItem.id) {
      _activeCodegenHistoryId = '';
    }
    _publishCodegenHistory();
    unawaited(_persistCodegenPreferences());
  }

  void _recordIncomingProcessMessage(ChatMessage message) {
    final meta = message.meta ?? const <String, dynamic>{};
    var origin = (meta['origin'] ?? '').toString().trim();
    if (!_isCodegenProcessMessage(message)) {
      return;
    }
    if (origin.isEmpty) {
      origin = 'app-process';
    }
    final sessionId = (meta['session_id'] ?? '').toString().trim();
    final requestId = (meta['request_id'] ?? '').toString().trim();
    final historyId = (meta['codegen_history_id'] ?? '').toString().trim();
    _appendProcessEntryToActiveCodegenHistory(
      message,
      message.content,
      origin: origin,
      historyId: historyId,
      sessionId: sessionId,
      requestId: requestId,
      mode: _codegenHistoryModeForProcessMessage(message),
    );
  }

  Future<void> _showCodegenHistoryDetails(CodegenHistoryItem item) async {
    await Navigator.of(context).push<void>(
      MaterialPageRoute(
        builder: (_) => CodegenTaskPage(
          historyListenable: _codegenHistoryNotifier,
          itemId: item.id,
          onReExecute: _reExecuteCodegenHistory,
          onApply: _applyCodegenHistoryItem,
          onToggleLock: _toggleCodegenHistoryLock,
        ),
      ),
    );
  }

  void _syncCodegenSelections() {
    final selectedCoding = _selectedCodingProject;
    if (selectedCoding == null && _codingProjects.isNotEmpty) {
      _selectedCodeProjectQualifiedName = _codingProjects.first.qualifiedName;
    }
    final codeTools = _selectedCodingProjectTools;
    if (codeTools.isEmpty) {
      _selectedCodeTool = '';
      _selectedClaudeSettings = '';
    } else {
      if (_selectedCodeTool.isEmpty || !codeTools.contains(_selectedCodeTool)) {
        final defaultTool =
            _selectedCodingProject?.defaultTool.trim().toLowerCase() ?? '';
        if (defaultTool.isNotEmpty && codeTools.contains(defaultTool)) {
          _selectedCodeTool = defaultTool;
        } else if (codeTools.contains('claudecode')) {
          _selectedCodeTool = 'claudecode';
        } else if (codeTools.contains('codex')) {
          _selectedCodeTool = 'codex';
        } else {
          _selectedCodeTool = codeTools.first;
        }
      }
      final settingsOptions = _selectedToolSettingsOptions;
      if (settingsOptions.isEmpty) {
        _selectedClaudeSettings = '';
      } else if (!settingsOptions.contains(_selectedClaudeSettings)) {
        final defaultSettings = _selectedCodingProject?.defaultSettings ?? '';
        if (defaultSettings.isNotEmpty &&
            settingsOptions.contains(defaultSettings)) {
          _selectedClaudeSettings = defaultSettings;
        } else {
          _selectedClaudeSettings = settingsOptions.first;
        }
      }
    }
    final selectedDeploy = _selectedDeployProject;
    if (selectedDeploy == null && _deployProjects.isNotEmpty) {
      _selectedDeployProjectQualifiedName = _deployProjects.first.qualifiedName;
    }
    final deployProject = _selectedDeployProject;
    if (deployProject == null) {
      _selectedDeployTarget = '';
      return;
    }
    if (deployProject.buildOnly) {
      _deployPackOnly = true;
    }
    if (_selectedDeployTarget.isNotEmpty &&
        deployProject.deployTargets.contains(_selectedDeployTarget)) {
      return;
    }
    _selectedDeployTarget = deployProject.deployTargets.isEmpty
        ? ''
        : deployProject.deployTargets.first;
  }

  Future<void> _loadCodegenProjects({bool silent = false}) async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      if (mounted) {
        setState(() {
          _codingProjects.clear();
          _deployProjects.clear();
          _codegenLoading = false;
          _codegenError = '';
        });
      }
      return;
    }
    if (mounted) {
      setState(() {
        _codegenLoading = true;
        if (!silent) {
          _codegenError = '';
        }
      });
    }
    try {
      final snapshot = await _runAuthed(
        'Load codegen projects',
        (client) => client.listCodegenProjects(),
      );
      if (!mounted) {
        _codingProjects
          ..clear()
          ..addAll(snapshot.codingProjects);
        _deployProjects
          ..clear()
          ..addAll(snapshot.deployProjects);
        _syncCodegenSelections();
        await _persistCodegenPreferences();
        return;
      }
      setState(() {
        _codingProjects
          ..clear()
          ..addAll(snapshot.codingProjects);
        _deployProjects
          ..clear()
          ..addAll(snapshot.deployProjects);
        _codegenError = '';
        _syncCodegenSelections();
      });
      await _persistCodegenPreferences();
    } catch (err) {
      if (mounted) {
        setState(() {
          _codegenError = _describeRequestError(
            err,
            operation: 'Load codegen projects',
          );
        });
      }
    } finally {
      if (mounted) {
        setState(() {
          _codegenLoading = false;
        });
      }
    }
  }

  void _setCodegenMode(CodegenLaunchMode mode) {
    if (_codegenMode == mode) {
      return;
    }
    setState(() {
      _codegenMode = mode;
    });
    unawaited(_persistCodegenPreferences());
  }

  void _handleCodeProjectChanged(String? qualifiedName) {
    final value = qualifiedName?.trim() ?? '';
    if (value.isEmpty || value == _selectedCodeProjectQualifiedName) {
      return;
    }
    setState(() {
      _selectedCodeProjectQualifiedName = value;
      _syncCodegenSelections();
    });
    unawaited(_persistCodegenPreferences());
  }

  void _handleCodeToolChanged(String? tool) {
    final value = tool?.trim().toLowerCase() ?? '';
    if (value.isEmpty || value == _selectedCodeTool) {
      return;
    }
    setState(() {
      _selectedCodeTool = value;
      _syncCodegenSelections();
    });
    unawaited(_persistCodegenPreferences());
  }

  void _handleClaudeSettingsChanged(String? settings) {
    final value = settings?.trim() ?? '';
    if (value == _selectedClaudeSettings) {
      return;
    }
    setState(() {
      _selectedClaudeSettings = value;
    });
    unawaited(_persistCodegenPreferences());
  }

  void _handleResumeLastSessionChanged(bool value) {
    if (value == _codegenResumeLastSession) {
      return;
    }
    setState(() {
      _codegenResumeLastSession = value;
    });
    unawaited(_persistCodegenPreferences());
  }

  void _handleDeployProjectChanged(String? qualifiedName) {
    final value = qualifiedName?.trim() ?? '';
    if (value.isEmpty) {
      return;
    }
    setState(() {
      _selectedDeployProjectQualifiedName = value;
      final deployProject = _selectedDeployProject;
      if (deployProject?.buildOnly == true) {
        _deployPackOnly = true;
      }
      _syncCodegenSelections();
    });
    unawaited(_persistCodegenPreferences());
  }

  void _handleDeployTargetChanged(String? target) {
    final value = target?.trim() ?? '';
    if (value == _selectedDeployTarget) {
      return;
    }
    setState(() {
      _selectedDeployTarget = value;
    });
    unawaited(_persistCodegenPreferences());
  }

  String _buildCodegenCommandPreview({
    CodegenLaunchMode? modeOverride,
    String? promptOverride,
    bool forceStartCommand = false,
    bool includeAutoDeploy = true,
  }) {
    final mode = modeOverride ?? _codegenMode;
    if (mode == CodegenLaunchMode.code) {
      final project = _selectedCodingProject;
      if (project == null) {
        return _codegenDebugBundleMode && !forceStartCommand
            ? '/cg debug <project@agent> --debug-id <debug_id> <request>'
            : '/cg start <project@agent> <request>';
      }
      final prompt = promptOverride ?? _codegenPromptController.text.trim();
      final parts = <String>[
        '/cg',
        _codegenDebugBundleMode && !forceStartCommand ? 'debug' : 'start',
        project.qualifiedName,
      ];
      if (_selectedCodeTool.isNotEmpty) {
        parts.add('@$_selectedCodeTool');
      }
      if (_selectedClaudeSettings.isNotEmpty) {
        parts.add('--settings');
        parts.add(_selectedClaudeSettings);
      }
      if (_codegenResumeLastSession && _selectedCodeToolSupportsResume) {
        parts.add('!resume');
      }
      if (_codegenDebugBundleMode && !forceStartCommand) {
        parts.add('--debug-id');
        parts.add('<debug_id>');
        parts.add('--debug-path');
        parts.add('<debug_path>');
      } else if (includeAutoDeploy && _codegenAutoDeploy) {
        parts.add('!deploy');
      }
      parts.add(prompt.isEmpty ? '<request>' : prompt);
      return parts.join(' ');
    }

    final project = _selectedDeployProject;
    if (project == null) {
      return '/cg deploy <project@agent>';
    }
    final parts = <String>['/cg', 'deploy', project.qualifiedName];
    if (_selectedDeployTarget.isNotEmpty) {
      parts.add('#$_selectedDeployTarget');
    }
    if (_deployPackOnly || project.buildOnly) {
      parts.add('!pack');
    }
    final deployArgs = _deployArgsController.text.trim();
    if (deployArgs.isNotEmpty) {
      parts.add(deployArgs);
    }
    return parts.join(' ');
  }

  CodegenActionRequest _buildCodegenActionFromDetails(
    CodegenHistoryCommandDetails details,
    String historyId, {
    String sourceCommand = '',
  }) {
    final qualified = details.projectQualifiedName.split('@');
    final project = qualified.isEmpty ? '' : qualified.first;
    final agent = qualified.length > 1 ? qualified.sublist(1).join('@') : '';
    final isDebug = sourceCommand.trim().startsWith('/cg debug ');
    final debugId =
        RegExp(r'--debug-id\s+(\S+)').firstMatch(sourceCommand)?.group(1) ?? '';
    final debugPath =
        RegExp(r'--debug-path\s+(\S+)').firstMatch(sourceCommand)?.group(1) ??
        '';
    var prompt = details.requestText;
    if (isDebug) {
      final lastDebugOption = debugPath.isNotEmpty
          ? '--debug-path $debugPath'
          : '--debug-id $debugId';
      final optionIndex = sourceCommand.indexOf(lastDebugOption);
      if (optionIndex >= 0) {
        prompt = sourceCommand
            .substring(optionIndex + lastDebugOption.length)
            .trim();
      }
    }
    return CodegenActionRequest(
      kind: details.mode == CodegenLaunchMode.deploy
          ? 'deploy'
          : (isDebug ? 'debug' : 'start'),
      project: project,
      agent: agent,
      historyId: historyId,
      tool: details.tool,
      settings: details.claudeSettings,
      prompt: prompt,
      resume: details.resumeLastSession,
      autoDeploy: details.autoDeploy,
      debugId: debugId,
      debugPath: debugPath,
      deployTarget: details.target,
      packOnly: details.packOnly,
      extraArgs: details.extraArgs.trim().isEmpty
          ? const <String>[]
          : <String>[details.extraArgs.trim()],
    );
  }

  CodegenActionRequest _buildCurrentCodegenAction(
    String historyId, {
    String promptOverride = '',
    String debugId = '',
    String debugPath = '',
    bool forceStart = false,
    bool includeAutoDeploy = true,
  }) {
    if (_codegenMode == CodegenLaunchMode.deploy && !forceStart) {
      final project = _selectedDeployProject!;
      final args = _deployArgsController.text.trim();
      return CodegenActionRequest(
        kind: 'deploy',
        project: project.name,
        agent: project.agent,
        historyId: historyId,
        deployTarget: _selectedDeployTarget,
        packOnly: _deployPackOnly || project.buildOnly,
        extraArgs: args.isEmpty ? const <String>[] : <String>[args],
      );
    }
    final project = _selectedCodingProject!;
    return CodegenActionRequest(
      kind: _codegenDebugBundleMode && !forceStart ? 'debug' : 'start',
      project: project.name,
      agent: project.agent,
      historyId: historyId,
      tool: _selectedCodeTool,
      settings: _selectedClaudeSettings,
      prompt: promptOverride.isEmpty
          ? _codegenPromptController.text.trim()
          : promptOverride,
      resume: _codegenResumeLastSession && _selectedCodeToolSupportsResume,
      autoDeploy: includeAutoDeploy && _codegenAutoDeploy,
      debugId: debugId,
      debugPath: debugPath,
    );
  }

  Future<void> _sendCodegenCommand() async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }

    var command = _buildCodegenCommandPreview().trim();
    if (_codegenMode == CodegenLaunchMode.code) {
      if (_selectedCodingProject == null) {
        _appendSystem('请先选择编码项目。');
        return;
      }
      if (_selectedCodeTool.isEmpty) {
        _appendSystem('请先选择编码工具。');
        return;
      }
      if (_selectedToolSettingsOptions.isNotEmpty &&
          _selectedClaudeSettings.isEmpty) {
        _appendSystem('请选择编码工具配置。');
        return;
      }
      if (_codegenPromptController.text.trim().isEmpty) {
        _appendSystem('请先输入编码需求。');
        return;
      }
    } else if (_selectedDeployProject == null) {
      _appendSystem('请先选择部署项目。');
      return;
    }

    FocusScope.of(context).unfocus();
    var debugId = '';
    var debugPath = '';
    if (_codegenMode == CodegenLaunchMode.code && _codegenDebugBundleMode) {
      final debugBundle = await _createCodegenDebugBundle();
      debugId = debugBundle['debug_id'] ?? '';
      debugPath = debugBundle['debug_path'] ?? '';
      if (debugId.isEmpty) {
        return;
      }
      command = command.replaceFirst('<debug_id>', debugId);
      if (debugPath.isEmpty) {
        command = command.replaceFirst(' --debug-path <debug_path>', '');
      } else {
        command = command.replaceFirst('<debug_path>', debugPath);
      }
    }
    final historyItem = _addCodegenHistory(command, _codegenMode);
    final action = _buildCurrentCodegenAction(
      historyItem.id,
      debugId: debugId,
      debugPath: debugPath,
    );
    setState(() {
      _codegenSending = true;
    });
    try {
      await _runAuthed('Send codegen command', (client) {
        return client.submitCodegenAction(action);
      });
      if (mounted) {
        setState(() {
          _status = _codegenMode == CodegenLaunchMode.code
              ? 'Code command sent'
              : 'Deploy command sent';
        });
      }
      _triggerCortanaContextualExpression('surprised');

      if (_codegenMode == CodegenLaunchMode.code) {
        _codegenPromptController.clear();
      }
      await _persistCodegenPreferences();
    } catch (err) {
      _markCodegenHistoryFailed(historyItem.id, err);
      _appendSystem(
        _describeRequestError(err, operation: 'Send codegen command'),
      );
    } finally {
      if (mounted) {
        setState(() {
          _codegenSending = false;
        });
      }
    }
  }

  Future<void> _sendCodegenCommitCommand() async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_selectedCodingProject == null) {
      _appendSystem('请先选择编码项目。');
      return;
    }
    if (_selectedCodeTool.isEmpty) {
      _appendSystem('请先选择编码工具。');
      return;
    }
    if (_selectedToolSettingsOptions.isNotEmpty &&
        _selectedClaudeSettings.isEmpty) {
      _appendSystem('请选择编码工具配置。');
      return;
    }

    FocusScope.of(context).unfocus();
    final command = _buildCodegenCommandPreview(
      modeOverride: CodegenLaunchMode.code,
      promptOverride: 'commit and push',
      forceStartCommand: true,
      includeAutoDeploy: false,
    ).trim();
    final historyItem = _addCodegenHistory(command, CodegenLaunchMode.code);
    final action = _buildCurrentCodegenAction(
      historyItem.id,
      promptOverride: 'commit and push',
      forceStart: true,
      includeAutoDeploy: false,
    );
    setState(() {
      _codegenSending = true;
    });
    try {
      await _runAuthed('Send codegen commit command', (client) {
        return client.submitCodegenAction(action);
      });
      if (mounted) {
        setState(() {
          _status = 'Code command sent';
        });
      }
      _triggerCortanaContextualExpression('surprised');
      await _persistCodegenPreferences();
    } catch (err) {
      _markCodegenHistoryFailed(historyItem.id, err);
      _appendSystem(
        _describeRequestError(err, operation: 'Send codegen commit command'),
      );
    } finally {
      if (mounted) {
        setState(() {
          _codegenSending = false;
        });
      }
    }
  }

  Future<void> _sendCodegenControlCommand(String kind) async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    final action = CodegenActionRequest(kind: kind, project: '');
    setState(() {
      _codegenSending = true;
    });
    try {
      await _runAuthed('Send codegen $kind command', (client) {
        return client.submitCodegenAction(action);
      });
      if (mounted) {
        setState(() {
          _status = kind == 'stop' ? '已请求停止编码会话' : '已请求查询编码进度';
        });
      }
    } catch (err) {
      _appendSystem(
        _describeRequestError(err, operation: 'Send codegen $kind command'),
      );
    } finally {
      if (mounted) {
        setState(() {
          _codegenSending = false;
        });
      }
    }
  }
}
