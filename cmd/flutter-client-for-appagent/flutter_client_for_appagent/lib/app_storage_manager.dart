// ignore_for_file: invalid_use_of_protected_member
part of 'main.dart';

class AppStoragePathUsage {
  const AppStoragePathUsage({
    required this.label,
    required this.path,
    required this.bytes,
    required this.fileCount,
    required this.exists,
  });

  final String label;
  final String path;
  final int bytes;
  final int fileCount;
  final bool exists;
}

class AppStorageUsage {
  const AppStorageUsage({
    required this.id,
    required this.label,
    required this.description,
    required this.icon,
    required this.bytes,
    required this.fileCount,
    required this.paths,
    this.canDelete = true,
    this.countsAsDiskUsage = true,
    this.deleteHint = '',
    this.error = '',
  });

  final String id;
  final String label;
  final String description;
  final IconData icon;
  final int bytes;
  final int fileCount;
  final List<AppStoragePathUsage> paths;
  final bool canDelete;
  final bool countsAsDiskUsage;
  final String deleteHint;
  final String error;

  bool get hasData => bytes > 0 || fileCount > 0;
}

class _AppStorageTarget {
  const _AppStorageTarget({required this.label, required this.path});

  final String label;
  final String path;
}

class _AppStorageMeasure {
  const _AppStorageMeasure({required this.bytes, required this.fileCount});

  final int bytes;
  final int fileCount;
}

extension _ChatPageStateStorageManager on _ChatPageState {
  Widget _buildAppStorageManagerCard() {
    final palette = _palette;
    final diskBytes = _appStorageUsages
        .where((usage) => usage.countsAsDiskUsage)
        .fold<int>(0, (sum, usage) => sum + usage.bytes);
    final scannedAt = _appStorageScannedAt;
    final subtitle = _appStorageScanning
        ? '正在扫描本地数据...'
        : _appStorageScanError.isNotEmpty
        ? '扫描失败: $_appStorageScanError'
        : _appStorageUsages.isEmpty
        ? '点击刷新，查看 Vosk、Live2D、语音、下载和缓存占用'
        : '已统计 ${_formatBytes(diskBytes)}'
              '${scannedAt == null ? '' : ' · ${_formatTime(scannedAt)}'}';

    return Container(
      decoration: BoxDecoration(
        color: palette.surfaceMuted.withValues(alpha: 0.5),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: palette.border.withValues(alpha: 0.55)),
      ),
      child: ExpansionTile(
        initiallyExpanded: _appStorageExpanded,
        onExpansionChanged: (expanded) {
          setState(() => _appStorageExpanded = expanded);
          if (expanded && _appStorageUsages.isEmpty && !_appStorageScanning) {
            unawaited(_refreshAppStorageUsage(silent: true));
          }
        },
        title: Text(
          '客户端数据管理',
          style: TextStyle(
            fontWeight: FontWeight.w600,
            color: palette.textPrimary,
          ),
        ),
        subtitle: Text(
          subtitle,
          style: TextStyle(fontSize: 12, color: palette.textMuted),
        ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
            child: _buildAppStorageManagerContent(),
          ),
        ],
      ),
    );
  }

  Widget _buildAppStorageManagerContent() {
    final palette = _palette;
    final totalBytes = _appStorageUsages
        .where((usage) => usage.countsAsDiskUsage)
        .fold<int>(0, (sum, usage) => sum + usage.bytes);
    final busy = _appStorageScanning || _appStorageDeletingCategory.isNotEmpty;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                '磁盘占用: ${_formatBytes(totalBytes)}',
                style: TextStyle(
                  color: palette.textPrimary,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ),
            TextButton.icon(
              onPressed: busy
                  ? null
                  : () => unawaited(_refreshAppStorageUsage()),
              icon: _appStorageScanning
                  ? SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: palette.accent,
                      ),
                    )
                  : const Icon(Icons.refresh_rounded, size: 16),
              label: const Text('刷新'),
            ),
          ],
        ),
        Text(
          '统计 Flutter 客户端自己写入的模型、音频、附件、缓存和历史数据；其他应用私有文件只展示大小，默认不删除。',
          style: TextStyle(
            color: palette.textSecondary,
            fontSize: 12,
            height: 1.35,
          ),
        ),
        if (_appStorageScanError.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(
            _appStorageScanError,
            style: TextStyle(color: palette.error, fontSize: 12),
          ),
        ],
        const SizedBox(height: 10),
        if (_appStorageUsages.isEmpty && !_appStorageScanning)
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: palette.surfaceRaised.withValues(alpha: 0.7),
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: palette.border),
            ),
            child: Text(
              '还没有扫描结果。',
              style: TextStyle(color: palette.textSecondary, fontSize: 13),
            ),
          )
        else
          ..._appStorageUsages.map(_buildAppStorageUsageTile),
      ],
    );
  }

  Widget _buildAppStorageUsageTile(AppStorageUsage usage) {
    final palette = _palette;
    final deleting = _appStorageDeletingCategory == usage.id;
    final categoryBusy = switch (usage.id) {
      'vosk' => _voskModelDownloading,
      'live2d' => _cortanaLive2dDownloading,
      'voice_audio' => _recording || _transcribingVoice,
      'downloads' => _sending,
      _ => false,
    };
    final busy =
        _appStorageScanning ||
        _appStorageDeletingCategory.isNotEmpty ||
        categoryBusy;
    final deleteEnabled = usage.canDelete && usage.hasData && !busy;
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: palette.surfaceRaised.withValues(alpha: 0.86),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: palette.border.withValues(alpha: 0.7)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(usage.icon, color: palette.accent, size: 20),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      usage.label,
                      style: TextStyle(
                        color: palette.textPrimary,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      usage.description,
                      style: TextStyle(
                        color: palette.textSecondary,
                        fontSize: 12,
                        height: 1.3,
                      ),
                    ),
                    if (usage.error.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text(
                        usage.error,
                        style: TextStyle(color: palette.error, fontSize: 12),
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(
                    _formatBytes(usage.bytes),
                    style: TextStyle(
                      color: palette.textPrimary,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                  Text(
                    usage.countsAsDiskUsage ? '${usage.fileCount} 个文件' : '内存',
                    style: TextStyle(color: palette.textMuted, fontSize: 11),
                  ),
                ],
              ),
              const SizedBox(width: 4),
              IconButton(
                tooltip: usage.canDelete
                    ? (usage.deleteHint.isEmpty
                          ? '删除 ${usage.label}'
                          : usage.deleteHint)
                    : '只统计，不自动删除',
                visualDensity: VisualDensity.compact,
                onPressed: deleteEnabled
                    ? () => unawaited(_confirmAndDeleteAppStorageUsage(usage))
                    : null,
                icon: deleting
                    ? SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: palette.accent,
                        ),
                      )
                    : const Icon(Icons.delete_outline_rounded, size: 20),
              ),
            ],
          ),
          if (usage.paths.isNotEmpty) ...[
            const SizedBox(height: 8),
            _buildAppStoragePathDetails(usage),
          ],
        ],
      ),
    );
  }

  Widget _buildAppStoragePathDetails(AppStorageUsage usage) {
    final palette = _palette;
    final visiblePaths = usage.paths.where((path) => path.exists).toList();
    if (visiblePaths.isEmpty) {
      return Text(
        '暂无已存在路径',
        style: TextStyle(color: palette.textMuted, fontSize: 11),
      );
    }
    return Column(
      children: visiblePaths
          .take(5)
          .map((path) {
            return Padding(
              padding: const EdgeInsets.only(bottom: 3),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 82,
                    child: Text(
                      path.label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: palette.textMuted,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      path.path,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: palette.textSecondary,
                        fontSize: 11,
                        fontFamily: 'monospace',
                      ),
                    ),
                  ),
                  const SizedBox(width: 6),
                  Text(
                    _formatBytes(path.bytes),
                    style: TextStyle(
                      color: palette.textPrimary,
                      fontSize: 11,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ),
            );
          })
          .toList(growable: false),
    );
  }

  Future<void> _refreshAppStorageUsage({bool silent = false}) async {
    if (_appStorageScanning) {
      return;
    }
    setState(() {
      _appStorageScanning = true;
      _appStorageScanError = '';
    });
    try {
      final usages = await _collectAppStorageUsages();
      if (!mounted) {
        return;
      }
      setState(() {
        _appStorageUsages = usages;
        _appStorageScannedAt = DateTime.now();
      });
      if (!silent) {
        _appendSystem('客户端数据占用已刷新。');
      }
    } catch (err, stack) {
      debugPrint('Scan app storage failed: $err\n$stack');
      if (!mounted) {
        return;
      }
      setState(() {
        _appStorageScanError = err.toString();
      });
      if (!silent) {
        _appendSystem('客户端数据占用扫描失败: $err');
      }
    } finally {
      if (mounted) {
        setState(() {
          _appStorageScanning = false;
        });
      }
    }
  }

  Future<List<AppStorageUsage>> _collectAppStorageUsages() async {
    final supportDir = await getApplicationSupportDirectory();
    final tempDir = await getTemporaryDirectory();
    final cacheDir = await _tryGetApplicationCacheDirectory();
    final documentsDir = await _tryGetApplicationDocumentsDirectory();

    final supportPath = supportDir.path;
    final tempPath = tempDir.path;
    final voskArchiveFile = await _getVoskArchiveFile();
    final voskArchive = _normalizeStoragePath(voskArchiveFile.path);
    final voskArchiveDir = _normalizeStoragePath(voskArchiveFile.parent.path);
    final live2dArchive = _joinStoragePath(tempPath, 'cortana-live2d.zip');
    final voiceTempTargets = await _topLevelStorageTargets(
      tempDir,
      label: '临时录音',
      include: (name) => name.startsWith('app_voice_'),
    );

    final knownTempPaths = <String>{
      voskArchive,
      '$voskArchive.part',
      live2dArchive,
      '$live2dArchive.part',
      ...voiceTempTargets.map((target) => target.path),
    };

    final knownSupportPaths = <String>{
      _joinStoragePath(supportPath, 'vosk-model-cn'),
      _joinStoragePath(supportPath, 'vosk-model-cn.extracting'),
      voskArchiveDir,
      _joinStoragePath(supportPath, 'cortana_live2d_models'),
      _joinStoragePath(supportPath, 'cortana_web_runtime'),
      _joinStoragePath(supportPath, 'voice_messages'),
      _joinStoragePath(supportPath, 'cortana_broadcasts'),
      _joinStoragePath(supportPath, 'downloads'),
      _joinStoragePath(supportPath, 'resource_downloads'),
    };

    final usages = <AppStorageUsage>[
      await _collectFileStorageUsage(
        id: 'vosk',
        label: 'Vosk 语音模型',
        description: '离线语音识别模型、未完成解压目录和下载临时包。',
        icon: Icons.record_voice_over_outlined,
        targets: <_AppStorageTarget>[
          _AppStorageTarget(
            label: '模型',
            path: _joinStoragePath(supportPath, 'vosk-model-cn'),
          ),
          _AppStorageTarget(
            label: '解压中',
            path: _joinStoragePath(supportPath, 'vosk-model-cn.extracting'),
          ),
          _AppStorageTarget(label: '压缩包', path: voskArchive),
          _AppStorageTarget(label: '未完成', path: '$voskArchive.part'),
        ],
        deleteHint: '删除 Vosk 模型和下载缓存',
      ),
      await _collectFileStorageUsage(
        id: 'live2d',
        label: 'Live2D 数据',
        description: '已安装 Cortana 模型、WebView 运行时副本和 Live2D 下载临时包。',
        icon: Icons.face_retouching_natural_outlined,
        targets: <_AppStorageTarget>[
          _AppStorageTarget(
            label: '模型库',
            path: _joinStoragePath(supportPath, 'cortana_live2d_models'),
          ),
          _AppStorageTarget(
            label: '运行时',
            path: _joinStoragePath(supportPath, 'cortana_web_runtime'),
          ),
          _AppStorageTarget(label: '压缩包', path: live2dArchive),
          _AppStorageTarget(label: '未完成', path: '$live2dArchive.part'),
        ],
        deleteHint: '删除 Live2D 模型和运行时副本',
      ),
      await _collectFileStorageUsage(
        id: 'voice_audio',
        label: '语音与音频',
        description: '语音消息、Cortana 播报缓存和残留临时录音。',
        icon: Icons.graphic_eq_rounded,
        targets: <_AppStorageTarget>[
          _AppStorageTarget(
            label: '语音消息',
            path: _joinStoragePath(supportPath, 'voice_messages'),
          ),
          _AppStorageTarget(
            label: '播报缓存',
            path: _joinStoragePath(supportPath, 'cortana_broadcasts'),
          ),
          ...voiceTempTargets,
        ],
        deleteHint: '删除本地语音和音频缓存',
      ),
      await _collectFileStorageUsage(
        id: 'downloads',
        label: '附件与 APK 下载',
        description: '聊天图片、文件、APK 和资源库下载到本机的副本。',
        icon: Icons.download_for_offline_outlined,
        targets: <_AppStorageTarget>[
          _AppStorageTarget(
            label: '附件',
            path: _joinStoragePath(supportPath, 'downloads'),
          ),
          _AppStorageTarget(
            label: '资源',
            path: _joinStoragePath(supportPath, 'resource_downloads'),
          ),
        ],
        deleteHint: '删除已下载附件和 APK',
      ),
      await _collectCacheStorageUsage(
        tempDir: tempDir,
        cacheDir: cacheDir,
        excludedTempPaths: knownTempPaths,
      ),
      await _collectHistoryStorageUsage(),
      _collectClientLogUsage(),
      await _collectOtherAppStorageUsage(
        supportDir: supportDir,
        documentsDir: documentsDir,
        knownSupportPaths: knownSupportPaths,
      ),
    ];

    usages.sort((a, b) {
      if (a.id == 'client_logs') return 1;
      if (b.id == 'client_logs') return -1;
      if (a.id == 'other_app_files') return 1;
      if (b.id == 'other_app_files') return -1;
      return b.bytes.compareTo(a.bytes);
    });
    return usages;
  }

  Future<AppStorageUsage> _collectFileStorageUsage({
    required String id,
    required String label,
    required String description,
    required IconData icon,
    required List<_AppStorageTarget> targets,
    required String deleteHint,
  }) async {
    var bytes = 0;
    var fileCount = 0;
    final paths = <AppStoragePathUsage>[];
    final errors = <String>[];
    for (final target in targets) {
      try {
        final usage = await _measureStoragePath(target.label, target.path);
        paths.add(usage);
        bytes += usage.bytes;
        fileCount += usage.fileCount;
      } catch (err) {
        errors.add('${target.label}: $err');
      }
    }
    return AppStorageUsage(
      id: id,
      label: label,
      description: description,
      icon: icon,
      bytes: bytes,
      fileCount: fileCount,
      paths: paths,
      deleteHint: deleteHint,
      error: errors.join('\n'),
    );
  }

  Future<AppStorageUsage> _collectCacheStorageUsage({
    required Directory tempDir,
    required Directory? cacheDir,
    required Set<String> excludedTempPaths,
  }) async {
    final tempTargets = await _topLevelStorageTargets(
      tempDir,
      label: '临时',
      include: (_) => true,
      excludePaths: excludedTempPaths,
    );
    final targets = <_AppStorageTarget>[
      if (cacheDir != null &&
          _normalizeStoragePath(cacheDir.path) !=
              _normalizeStoragePath(tempDir.path))
        _AppStorageTarget(label: '缓存', path: cacheDir.path),
      ...tempTargets,
    ];
    return _collectFileStorageUsage(
      id: 'cache_temp',
      label: '临时与缓存',
      description: 'Flutter cache、未归类临时文件和可重新生成的数据。',
      icon: Icons.cleaning_services_outlined,
      targets: targets,
      deleteHint: '删除临时和缓存文件',
    );
  }

  Future<AppStorageUsage> _collectHistoryStorageUsage() async {
    var bytes = 0;
    var count = 0;
    final prefs = await SharedPreferences.getInstance();
    for (final key in prefs.getKeys()) {
      if (!_isHistoryStorageKey(key)) {
        continue;
      }
      final value = prefs.get(key);
      bytes += utf8.encode('$key=${value ?? ''}').length;
      count++;
    }
    try {
      final secureValues = await _secureStorage.readAll();
      for (final entry in secureValues.entries) {
        if (!entry.key.startsWith(_historyBackupStoragePrefix)) {
          continue;
        }
        bytes += utf8.encode('${entry.key}=${entry.value}').length;
        count++;
      }
    } catch (err) {
      return AppStorageUsage(
        id: 'history',
        label: '聊天与编码历史',
        description: '聊天记录、历史备份和编码发布历史。',
        icon: Icons.history_rounded,
        bytes: bytes,
        fileCount: count,
        paths: const <AppStoragePathUsage>[],
        deleteHint: '删除聊天与编码历史',
        error: '读取安全历史备份失败: $err',
      );
    }
    return AppStorageUsage(
      id: 'history',
      label: '聊天与编码历史',
      description: '聊天记录、历史备份和编码发布历史。',
      icon: Icons.history_rounded,
      bytes: bytes,
      fileCount: count,
      paths: const <AppStoragePathUsage>[],
      deleteHint: '删除聊天与编码历史',
    );
  }

  AppStorageUsage _collectClientLogUsage() {
    final bytes = flutterClientLogs.fold<int>(
      0,
      (sum, entry) => sum + utf8.encode(entry.message).length,
    );
    final debugBytes = _llmDebugEvents.fold<int>(
      0,
      (sum, event) => sum + utf8.encode(event.detailText).length,
    );
    return AppStorageUsage(
      id: 'client_logs',
      label: '客户端日志',
      description: '当前进程内存里的 Flutter 客户端日志和 LLM 调试事件，通常不是 8G 来源。',
      icon: Icons.article_outlined,
      bytes: bytes + debugBytes,
      fileCount: flutterClientLogs.length + _llmDebugEvents.length,
      paths: const <AppStoragePathUsage>[],
      countsAsDiskUsage: false,
      deleteHint: '清空内存日志',
    );
  }

  Future<AppStorageUsage> _collectOtherAppStorageUsage({
    required Directory supportDir,
    required Directory? documentsDir,
    required Set<String> knownSupportPaths,
  }) async {
    final targets = <_AppStorageTarget>[
      ...await _topLevelStorageTargets(
        supportDir,
        label: 'Support',
        include: (_) => true,
        excludePaths: knownSupportPaths,
      ),
      if (documentsDir != null &&
          _normalizeStoragePath(documentsDir.path) !=
              _normalizeStoragePath(supportDir.path))
        _AppStorageTarget(label: 'Documents', path: documentsDir.path),
    ];
    final usage = await _collectFileStorageUsage(
      id: 'other_app_files',
      label: '其他应用文件',
      description: '未归入上面分类的应用私有文件。为避免误删登录、插件或平台数据，此项只统计。',
      icon: Icons.folder_copy_outlined,
      targets: targets,
      deleteHint: '',
    );
    return AppStorageUsage(
      id: usage.id,
      label: usage.label,
      description: usage.description,
      icon: usage.icon,
      bytes: usage.bytes,
      fileCount: usage.fileCount,
      paths: usage.paths,
      canDelete: false,
      error: usage.error,
    );
  }

  Future<AppStoragePathUsage> _measureStoragePath(
    String label,
    String path,
  ) async {
    final normalizedPath = _normalizeStoragePath(path);
    final type = await FileSystemEntity.type(
      normalizedPath,
      followLinks: false,
    );
    if (type == FileSystemEntityType.notFound) {
      return AppStoragePathUsage(
        label: label,
        path: normalizedPath,
        bytes: 0,
        fileCount: 0,
        exists: false,
      );
    }
    final measure = switch (type) {
      FileSystemEntityType.directory => await _measureStorageDirectory(
        Directory(normalizedPath),
      ),
      FileSystemEntityType.file => _AppStorageMeasure(
        bytes: await File(normalizedPath).length(),
        fileCount: 1,
      ),
      FileSystemEntityType.link => const _AppStorageMeasure(
        bytes: 0,
        fileCount: 0,
      ),
      FileSystemEntityType.notFound => const _AppStorageMeasure(
        bytes: 0,
        fileCount: 0,
      ),
      _ => const _AppStorageMeasure(bytes: 0, fileCount: 0),
    };
    return AppStoragePathUsage(
      label: label,
      path: normalizedPath,
      bytes: measure.bytes,
      fileCount: measure.fileCount,
      exists: true,
    );
  }

  Future<_AppStorageMeasure> _measureStorageDirectory(Directory dir) async {
    var bytes = 0;
    var fileCount = 0;
    if (!await dir.exists()) {
      return const _AppStorageMeasure(bytes: 0, fileCount: 0);
    }
    await for (final entity in dir.list(recursive: true, followLinks: false)) {
      try {
        if (entity is File) {
          bytes += await entity.length();
          fileCount++;
        }
      } catch (_) {}
    }
    return _AppStorageMeasure(bytes: bytes, fileCount: fileCount);
  }

  Future<List<_AppStorageTarget>> _topLevelStorageTargets(
    Directory dir, {
    required String label,
    required bool Function(String name) include,
    Set<String> excludePaths = const <String>{},
  }) async {
    if (!await dir.exists()) {
      return const <_AppStorageTarget>[];
    }
    final targets = <_AppStorageTarget>[];
    await for (final entity in dir.list(recursive: false, followLinks: false)) {
      final path = _normalizeStoragePath(entity.path);
      if (_pathIsUnderAny(path, excludePaths)) {
        continue;
      }
      final name = _storageFileName(path);
      if (!include(name)) {
        continue;
      }
      targets.add(_AppStorageTarget(label: label, path: path));
    }
    targets.sort((a, b) => a.path.compareTo(b.path));
    return targets;
  }

  Future<void> _confirmAndDeleteAppStorageUsage(AppStorageUsage usage) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          title: Text('删除${usage.label}？'),
          content: Text(
            '将删除 ${_formatBytes(usage.bytes)} / ${usage.fileCount} 个文件。'
            '${usage.id == 'history' ? '\n聊天与编码历史会从本机移除。' : ''}',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.of(dialogContext).pop(true),
              child: const Text('删除'),
            ),
          ],
        );
      },
    );
    if (confirmed != true) {
      return;
    }
    await _deleteAppStorageUsage(usage);
  }

  Future<void> _deleteAppStorageUsage(AppStorageUsage usage) async {
    if (_appStorageDeletingCategory.isNotEmpty) {
      return;
    }
    setState(() {
      _appStorageDeletingCategory = usage.id;
    });
    try {
      switch (usage.id) {
        case 'vosk':
          await _deleteVoskStorage();
          break;
        case 'live2d':
          await _deleteLive2dStorage();
          break;
        case 'voice_audio':
          await _deleteVoiceAudioStorage();
          break;
        case 'downloads':
          await _deleteDownloadStorage();
          break;
        case 'cache_temp':
          await _deleteCacheTempStorage();
          break;
        case 'history':
          await _deleteHistoryStorage();
          break;
        case 'client_logs':
          flutterClientLogs.clear();
          _llmDebugEvents.clear();
          break;
        default:
          return;
      }
      if (!mounted) {
        return;
      }
      _appendSystem('已删除${usage.label}: ${_formatBytes(usage.bytes)}');
      await _refreshAppStorageUsage(silent: true);
    } catch (err, stack) {
      debugPrint('Delete app storage failed: $err\n$stack');
      if (mounted) {
        _appendSystem('删除${usage.label}失败: $err');
      }
    } finally {
      if (mounted) {
        setState(() {
          _appStorageDeletingCategory = '';
        });
      }
    }
  }

  Future<void> _deleteVoskStorage() async {
    final prefs = await SharedPreferences.getInstance();
    final modelPath = await _getLocalVoskModelPath();
    final archiveFile = await _getVoskArchiveFile();
    final partFile = await _getVoskArchivePartFile();
    final tempModelDir = await _getVoskExtractionTempDir();
    await _deleteDirectoryIfExists(Directory(modelPath));
    await _deleteDirectoryIfExists(tempModelDir);
    await _deleteFileIfExists(archiveFile);
    await _deleteFileIfExists(partFile);
    await prefs.remove('vosk_model_path');
    await prefs.remove(_voskDownloadProgressKey);
    await prefs.remove(_voskDownloadBytesKey);
    await prefs.remove(_voskActiveDownloadUrlKey);
    if (mounted) {
      setState(() {
        _speechReady = _systemSpeechReady;
        _useLocalVosk = false;
        _voskModelDownloadProgress = 0.0;
        _voskModelDownloadError = null;
      });
    }
  }

  Future<void> _deleteLive2dStorage() async {
    final prefs = await SharedPreferences.getInstance();
    final rootDir = await _getCortanaLive2dRootDir();
    final archiveFile = await _getCortanaLive2dArchiveFile();
    final supportDir = await getApplicationSupportDirectory();
    final runtimeDir = Directory(
      _joinStoragePath(supportDir.path, 'cortana_web_runtime'),
    );
    await _deleteDirectoryIfExists(rootDir);
    await _deleteDirectoryIfExists(runtimeDir);
    await _deleteFileIfExists(archiveFile);
    await _deleteFileIfExists(File('${archiveFile.path}.part'));
    await prefs.remove(_cortanaLive2dModelsKey);
    await prefs.remove(_cortanaSelectedLive2dModelKey);
    await prefs.remove(_cortanaLive2dViewTransformsKey);
    if (mounted) {
      setState(() {
        _cortanaLive2dModels = <CortanaLive2dModelInfo>[];
        _selectedCortanaLive2dModelId = '';
        _cortanaLive2dViewTransforms = <String, CortanaModelViewTransform>{};
        _cortanaLive2dDownloadProgress = 0.0;
        _cortanaLive2dDownloadError = '';
      });
    }
  }

  Future<void> _deleteVoiceAudioStorage() async {
    final supportDir = await getApplicationSupportDirectory();
    final tempDir = await getTemporaryDirectory();
    await _deleteDirectoryIfExists(
      Directory(_joinStoragePath(supportDir.path, 'voice_messages')),
    );
    await _deleteDirectoryIfExists(
      Directory(_joinStoragePath(supportDir.path, 'cortana_broadcasts')),
    );
    final tempVoiceTargets = await _topLevelStorageTargets(
      tempDir,
      label: '临时录音',
      include: (name) => name.startsWith('app_voice_'),
    );
    for (final target in tempVoiceTargets) {
      await _deleteStoragePath(target.path);
    }
  }

  Future<void> _deleteDownloadStorage() async {
    final supportDir = await getApplicationSupportDirectory();
    await _deleteDirectoryIfExists(
      Directory(_joinStoragePath(supportDir.path, 'downloads')),
    );
    await _deleteDirectoryIfExists(
      Directory(_joinStoragePath(supportDir.path, 'resource_downloads')),
    );
  }

  Future<void> _deleteCacheTempStorage() async {
    final tempDir = await getTemporaryDirectory();
    final cacheDir = await _tryGetApplicationCacheDirectory();
    final voskArchive = _joinStoragePath(tempDir.path, 'vosk-model-cn.zip');
    final live2dArchive = _joinStoragePath(tempDir.path, 'cortana-live2d.zip');
    final tempVoiceTargets = await _topLevelStorageTargets(
      tempDir,
      label: '临时录音',
      include: (name) => name.startsWith('app_voice_'),
    );
    final excludedTempPaths = <String>{
      voskArchive,
      '$voskArchive.part',
      live2dArchive,
      '$live2dArchive.part',
      ...tempVoiceTargets.map((target) => target.path),
    };
    if (cacheDir != null) {
      await _deleteDirectoryIfExists(cacheDir);
      await cacheDir.create(recursive: true);
    }
    if (await tempDir.exists()) {
      await for (final entity in tempDir.list(
        recursive: false,
        followLinks: false,
      )) {
        if (_pathIsUnderAny(entity.path, excludedTempPaths)) {
          continue;
        }
        await _deleteStoragePath(entity.path);
      }
      await tempDir.create(recursive: true);
    }
  }

  Future<void> _deleteHistoryStorage() async {
    final prefs = await SharedPreferences.getInstance();
    for (final key in prefs.getKeys().where(_isHistoryStorageKey).toList()) {
      await prefs.remove(key);
    }
    try {
      final secureValues = await _secureStorage.readAll();
      for (final key in secureValues.keys.where(
        (key) =>
            key.startsWith(_historyBackupStoragePrefix) ||
            key == _codegenHistoryBackupKey,
      )) {
        await _secureStorage.delete(key: key);
      }
    } catch (err) {
      debugPrint('Clear secure history failed: $err');
    }
    if (mounted) {
      setState(() {
        _historyByScope.clear();
        _seenMessageIds.clear();
        _codegenHistory = <CodegenHistoryItem>[];
        _activeCodegenHistoryId = '';
      });
    }
  }

  Future<void> _deleteStoragePath(String path) async {
    final normalizedPath = _normalizeStoragePath(path);
    final type = await FileSystemEntity.type(
      normalizedPath,
      followLinks: false,
    );
    if (type == FileSystemEntityType.notFound) {
      return;
    }
    if (type == FileSystemEntityType.directory) {
      await Directory(normalizedPath).delete(recursive: true);
      return;
    }
    await File(normalizedPath).delete();
  }

  bool _isHistoryStorageKey(String key) {
    return key.startsWith(_historyStoragePrefix) ||
        key.startsWith(_lastReadAtStoragePrefix) ||
        key == _codegenHistoryKey ||
        key == _codegenHistoryBackupKey ||
        key == _codegenHistoryLastBackupAtKey;
  }

  Future<Directory?> _tryGetApplicationCacheDirectory() async {
    try {
      return await getApplicationCacheDirectory();
    } catch (_) {
      return null;
    }
  }

  Future<Directory?> _tryGetApplicationDocumentsDirectory() async {
    try {
      return await getApplicationDocumentsDirectory();
    } catch (_) {
      return null;
    }
  }

  String _joinStoragePath(String base, String child) {
    final trimmedBase = base.endsWith(Platform.pathSeparator)
        ? base.substring(0, base.length - 1)
        : base;
    return '$trimmedBase${Platform.pathSeparator}$child';
  }

  String _normalizeStoragePath(String path) {
    final normalized = File(path).absolute.path;
    if (normalized.length > 1 && normalized.endsWith(Platform.pathSeparator)) {
      return normalized.substring(0, normalized.length - 1);
    }
    return normalized;
  }

  bool _pathIsUnderAny(String path, Set<String> roots) {
    final normalizedPath = _normalizeStoragePath(path);
    for (final root in roots) {
      final normalizedRoot = _normalizeStoragePath(root);
      if (normalizedPath == normalizedRoot ||
          normalizedPath.startsWith(
            '$normalizedRoot${Platform.pathSeparator}',
          )) {
        return true;
      }
    }
    return false;
  }

  String _storageFileName(String path) {
    final normalized = path.replaceAll('\\', '/');
    final slash = normalized.lastIndexOf('/');
    return slash >= 0 ? normalized.substring(slash + 1) : normalized;
  }
}
