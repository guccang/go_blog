// ignore_for_file: invalid_use_of_protected_member
part of 'main.dart';

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
    } catch (_) {
      // Ignore local preference persistence failures.
    }
  }

  Future<void> _loadCodegenHistory() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final historyJson = prefs.getString(_codegenHistoryKey);
      if (historyJson != null && historyJson.isNotEmpty) {
        final List<dynamic> historyList = jsonDecode(historyJson);
        setState(() {
          _codegenHistory = historyList
              .map(
                (item) => _normalizeCodegenHistoryItem(
                  CodegenHistoryItem.fromJson(item as Map<String, dynamic>),
                ),
              )
              .toList();
        });
      }
    } catch (_) {
      // Ignore history load failures.
    }
  }

  String _newCodegenHistoryId() =>
      'cg_${DateTime.now().microsecondsSinceEpoch.toString()}';

  CodegenHistoryItem _normalizeCodegenHistoryItem(CodegenHistoryItem item) {
    if (item.id.trim().isNotEmpty) {
      return item;
    }
    return CodegenHistoryItem(
      id: _newCodegenHistoryId(),
      timestamp: item.timestamp,
      command: item.command,
      mode: item.mode,
      locked: item.locked,
      completed: item.completed,
      processEntries: item.processEntries,
    );
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
    _activeCodegenHistoryId = item.id;
    unawaited(_persistCodegenPreferences());
    return item;
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
    _appendOutgoing(item.command);
    setState(() {
      _codegenSending = true;
    });
    _runAuthed('Re-execute codegen command', (client) {
          return client.sendMessage(item.command);
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
          _appendSystem('命令已重新发送，执行进度会继续在聊天流中返回。');
          _addCodegenHistory(item.command, item.mode);
        })
        .catchError((err) {
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

  void _toggleCodegenHistoryLock(CodegenHistoryItem item) {
    setState(() {
      final idx = _codegenHistory.indexOf(item);
      if (idx == -1) return;
      _codegenHistory[idx] = item.copyWith(locked: !item.locked);
    });
    unawaited(_persistCodegenPreferences());
  }

  CodegenHistoryItem? _findActiveCodegenHistoryItem() {
    if (_activeCodegenHistoryId.isNotEmpty) {
      for (final item in _codegenHistory) {
        if (item.id == _activeCodegenHistoryId) {
          return item;
        }
      }
    }
    for (final item in _codegenHistory) {
      if (!item.completed) {
        _activeCodegenHistoryId = item.id;
        return item;
      }
    }
    return null;
  }

  bool _hasPendingCodegenHistoryExecution() =>
      _findActiveCodegenHistoryItem() != null;

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
    String sessionId = '',
  }) {
    final normalized = content.trimRight();
    if (normalized.trim().isEmpty) {
      return;
    }
    final activeItem = _findActiveCodegenHistoryItem();
    if (activeItem == null) {
      return;
    }
    final entries = List<CodegenProcessEntry>.from(activeItem.processEntries);
    final last = entries.isNotEmpty ? entries.last : null;
    if (last != null &&
        last.content == normalized &&
        last.origin == origin &&
        last.sessionId == sessionId) {
      return;
    }
    entries.add(
      CodegenProcessEntry(
        timestamp: message.timestamp,
        content: normalized,
        origin: origin,
        sessionId: sessionId,
      ),
    );
    final updatedItem = activeItem.copyWith(
      processEntries: entries,
      completed: _isTerminalCodegenProcessMessage(message),
    );
    setState(() {
      final idx = _codegenHistory.indexWhere(
        (item) => item.id == activeItem.id,
      );
      if (idx == -1) {
        return;
      }
      _codegenHistory[idx] = updatedItem;
    });
    if (updatedItem.completed) {
      _activeCodegenHistoryId = '';
    } else {
      _activeCodegenHistoryId = updatedItem.id;
    }
    unawaited(_persistCodegenPreferences());
  }

  void _recordIncomingProcessMessage(ChatMessage message) {
    final meta = message.meta ?? const <String, dynamic>{};
    final origin = (meta['origin'] ?? '').toString().trim();
    if (origin != 'app-process') {
      return;
    }
    final sessionId = (meta['session_id'] ?? '').toString().trim();
    _appendProcessEntryToActiveCodegenHistory(
      message,
      message.content,
      origin: origin,
      sessionId: sessionId,
    );
  }

  Future<void> _showCodegenHistoryDetails(CodegenHistoryItem item) async {
    final details = CodegenHistoryCommandDetails.parse(item);
    final palette = _palette;
    final processTranscript = item.processEntries.isEmpty
        ? ''
        : item.processEntries
              .map((entry) {
                final hh = entry.timestamp.hour.toString().padLeft(2, '0');
                final mm = entry.timestamp.minute.toString().padLeft(2, '0');
                final ss = entry.timestamp.second.toString().padLeft(2, '0');
                return '[$hh:$mm:$ss] ${entry.content}';
              })
              .join('\n\n');
    final exactTime =
        '${item.timestamp.year}-${item.timestamp.month.toString().padLeft(2, '0')}-${item.timestamp.day.toString().padLeft(2, '0')} '
        '${item.timestamp.hour.toString().padLeft(2, '0')}:${item.timestamp.minute.toString().padLeft(2, '0')}:${item.timestamp.second.toString().padLeft(2, '0')}';
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) {
        final bottomInset = MediaQuery.of(context).viewInsets.bottom;
        return Padding(
          padding: EdgeInsets.fromLTRB(16, 16, 16, bottomInset + 16),
          child: Container(
            decoration: BoxDecoration(
              color: palette.surface,
              borderRadius: BorderRadius.circular(28),
              border: Border.all(color: palette.border),
            ),
            child: SafeArea(
              top: false,
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(20, 20, 20, 20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 10,
                            vertical: 4,
                          ),
                          decoration: BoxDecoration(
                            color: _codegenModeColor(
                              item.mode,
                            ).withValues(alpha: 0.2),
                            borderRadius: BorderRadius.circular(999),
                          ),
                          child: Text(
                            _codegenModeLabel(item.mode),
                            style: TextStyle(
                              color: _codegenModeColor(item.mode),
                              fontSize: 12,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ),
                        if (item.locked) ...[
                          const SizedBox(width: 8),
                          Icon(
                            Icons.lock_rounded,
                            size: 16,
                            color: palette.textSecondary,
                          ),
                        ],
                        const Spacer(),
                        IconButton(
                          onPressed: () {
                            _toggleCodegenHistoryLock(item);
                            Navigator.of(context).pop();
                            _showCodegenHistoryDetails(
                              item.copyWith(locked: !item.locked),
                            );
                          },
                          icon: Icon(
                            item.locked
                                ? Icons.lock_rounded
                                : Icons.lock_open_rounded,
                            size: 20,
                          ),
                          tooltip: item.locked ? '取消锁定' : '锁定',
                        ),
                        IconButton(
                          onPressed: () => Navigator.of(context).pop(),
                          icon: const Icon(Icons.close_rounded),
                          tooltip: '关闭',
                        ),
                      ],
                    ),
                    const SizedBox(height: 8),
                    Text(
                      '历史记录详情',
                      style: TextStyle(
                        color: palette.textPrimary,
                        fontSize: 20,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    const SizedBox(height: 6),
                    Text(
                      '记录时间：$exactTime',
                      style: TextStyle(
                        color: palette.textSecondary,
                        fontSize: 13,
                      ),
                    ),
                    const SizedBox(height: 18),
                    if (details.mode != CodegenLaunchMode.backup)
                      _buildHistoryDetailRow(
                        '项目',
                        details.projectQualifiedName,
                      ),
                    if (details.mode == CodegenLaunchMode.backup)
                      _buildHistoryDetailRow(
                        '备份类型',
                        details.requestText == 'full' ? '全量备份' : '增量备份',
                      ),
                    if (details.mode == CodegenLaunchMode.code)
                      _buildHistoryDetailRow(
                        '工具',
                        details.tool.isEmpty ? '默认' : details.tool,
                      ),
                    if (details.mode == CodegenLaunchMode.code &&
                        details.claudeSettings.isNotEmpty)
                      _buildHistoryDetailRow(
                        'Settings',
                        details.claudeSettings,
                      ),
                    if (details.mode == CodegenLaunchMode.code)
                      _buildHistoryDetailRow(
                        '自动发布',
                        details.autoDeploy ? '是' : '否',
                      ),
                    if (details.mode == CodegenLaunchMode.code)
                      _buildHistoryDetailRow('需求', details.requestText),
                    if (details.mode == CodegenLaunchMode.deploy)
                      _buildHistoryDetailRow(
                        '部署目标',
                        details.target.isEmpty ? '未指定' : details.target,
                      ),
                    if (details.mode == CodegenLaunchMode.deploy)
                      _buildHistoryDetailRow(
                        '仅打包',
                        details.packOnly ? '是' : '否',
                      ),
                    if (details.mode == CodegenLaunchMode.deploy)
                      _buildHistoryDetailRow(
                        '附加参数',
                        details.extraArgs.isEmpty ? '无' : details.extraArgs,
                      ),
                    _buildHistoryDetailRow(
                      '执行状态',
                      item.completed ? '已结束' : '进行中/未确认结束',
                    ),
                    _buildHistoryDetailRow(
                      '过程消息数',
                      item.processEntries.length.toString(),
                    ),
                    _buildHistoryDetailRow('完整命令', item.command, mono: true),
                    if (processTranscript.isNotEmpty)
                      _buildHistoryDetailRow(
                        '执行过程',
                        processTranscript,
                        mono: true,
                      ),
                    const SizedBox(height: 20),
                    SizedBox(
                      width: double.infinity,
                      child: FilledButton.icon(
                        onPressed: () {
                          Navigator.of(context).pop();
                          _reExecuteCodegenHistory(item);
                        },
                        icon: const Icon(Icons.play_arrow_rounded),
                        label: const Text('直接执行'),
                        style: FilledButton.styleFrom(
                          backgroundColor: Colors.green,
                        ),
                      ),
                    ),
                    const SizedBox(height: 8),
                    SizedBox(
                      width: double.infinity,
                      child: FilledButton.icon(
                        onPressed: () {
                          Navigator.of(context).pop();
                          _applyCodegenHistoryItem(item);
                        },
                        icon: const Icon(Icons.edit_note_rounded),
                        label: const Text('回填到当前表单'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildHistoryDetailRow(
    String label,
    String value, {
    bool mono = false,
  }) {
    final palette = _palette;
    final displayValue = value.trim().isEmpty ? '未识别' : value.trim();
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: TextStyle(
              color: palette.textSecondary,
              fontSize: 12,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 6),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
            decoration: BoxDecoration(
              color: palette.surfaceMuted,
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: palette.border),
            ),
            child: SelectableText(
              displayValue,
              style: TextStyle(
                color: palette.textPrimary,
                fontSize: 13,
                height: 1.5,
                fontFamily: mono ? 'monospace' : null,
              ),
            ),
          ),
        ],
      ),
    );
  }

  String _codegenModeLabel(CodegenLaunchMode mode) {
    switch (mode) {
      case CodegenLaunchMode.code:
        return '编码';
      case CodegenLaunchMode.deploy:
        return '发布';
      case CodegenLaunchMode.backup:
        return '备份';
    }
  }

  Color _codegenModeColor(CodegenLaunchMode mode) {
    switch (mode) {
      case CodegenLaunchMode.code:
        return Colors.blue;
      case CodegenLaunchMode.deploy:
        return Colors.green;
      case CodegenLaunchMode.backup:
        return Colors.deepPurple;
    }
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
    if (_codegenMode == CodegenLaunchMode.code && _codegenDebugBundleMode) {
      final debugBundle = await _createCodegenDebugBundle();
      final debugID = debugBundle['debug_id'] ?? '';
      final debugPath = debugBundle['debug_path'] ?? '';
      if (debugID.isEmpty) {
        return;
      }
      command = command.replaceFirst('<debug_id>', debugID);
      if (debugPath.isEmpty) {
        command = command.replaceFirst(' --debug-path <debug_path>', '');
      } else {
        command = command.replaceFirst('<debug_path>', debugPath);
      }
    }
    _appendOutgoing(command);
    setState(() {
      _codegenSending = true;
    });
    try {
      await _runAuthed('Send codegen command', (client) {
        return client.sendMessage(command);
      });
      if (mounted) {
        setState(() {
          _status = _codegenMode == CodegenLaunchMode.code
              ? 'Code command sent'
              : 'Deploy command sent';
        });
      }
      _triggerCortanaContextualExpression('surprised');
      _appendSystem('命令已发送，执行进度会继续在聊天流中返回。');

      // 添加到历史记录
      _addCodegenHistory(command, _codegenMode);

      if (_codegenMode == CodegenLaunchMode.code) {
        _codegenPromptController.clear();
      }
      await _persistCodegenPreferences();
    } catch (err) {
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
    _appendOutgoing(command);
    setState(() {
      _codegenSending = true;
    });
    try {
      await _runAuthed('Send codegen commit command', (client) {
        return client.sendMessage(command);
      });
      if (mounted) {
        setState(() {
          _status = 'Code command sent';
        });
      }
      _triggerCortanaContextualExpression('surprised');
      _appendSystem('git 提交命令已发送，执行进度会继续在聊天流中返回。');
      _addCodegenHistory(command, CodegenLaunchMode.code);
      await _persistCodegenPreferences();
    } catch (err) {
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
}
