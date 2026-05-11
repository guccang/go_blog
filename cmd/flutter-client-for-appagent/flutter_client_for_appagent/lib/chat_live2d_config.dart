// ignore_for_file: invalid_use_of_protected_member
part of 'main.dart';

extension _ChatPageStateLive2dConfig on _ChatPageState {
  Future<String> _getLocalVoskModelPath() async {
    final supportDir = await getApplicationSupportDirectory();
    return '${supportDir.path}${Platform.pathSeparator}vosk-model-cn';
  }

  Future<File> _getVoskArchiveFile() async {
    final tempDir = await getTemporaryDirectory();
    return File('${tempDir.path}${Platform.pathSeparator}vosk-model-cn.zip');
  }

  Future<File> _getVoskArchivePartFile() async {
    final archiveFile = await _getVoskArchiveFile();
    return File('${archiveFile.path}.part');
  }

  Future<Directory> _getVoskExtractionTempDir() async {
    final modelPath = await _getLocalVoskModelPath();
    return Directory('$modelPath.__extracting__');
  }

  Future<void> _migrateLegacyVoskPartialArchive(SharedPreferences prefs) async {
    final savedProgress = await _getVoskDownloadProgress();
    if (savedProgress <= 0 || savedProgress >= 1.0) {
      return;
    }
    final archiveFile = await _getVoskArchiveFile();
    final partFile = await _getVoskArchivePartFile();
    if (!await archiveFile.exists() || await partFile.exists()) {
      return;
    }
    try {
      await archiveFile.rename(partFile.path);
    } catch (_) {
      await archiveFile.copy(partFile.path);
      await archiveFile.delete();
    }
    final partialBytes = await partFile.length();
    await prefs.setInt(_voskDownloadBytesKey, partialBytes);
  }

  Future<void> _deleteDirectoryIfExists(Directory dir) async {
    if (await dir.exists()) {
      await dir.delete(recursive: true);
    }
  }

  Future<void> _deleteFileIfExists(File file) async {
    if (await file.exists()) {
      await file.delete();
    }
  }

  Future<Directory> _getCortanaLive2dRootDir() async {
    final supportDir = await getApplicationSupportDirectory();
    return Directory(
      '${supportDir.path}${Platform.pathSeparator}cortana_live2d_models',
    );
  }

  Future<File> _getCortanaLive2dArchiveFile() async {
    final tempDir = await getTemporaryDirectory();
    return File('${tempDir.path}${Platform.pathSeparator}cortana-live2d.zip');
  }

  Future<void> _restoreCortanaLive2dModels() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final raw = prefs.getString(_cortanaLive2dModelsKey)?.trim() ?? '';
      final selected = prefs.getString(_cortanaSelectedLive2dModelKey) ?? '';
      final rawTransforms =
          prefs.getString(_cortanaLive2dViewTransformsKey)?.trim() ?? '';
      final restored = <CortanaLive2dModelInfo>[];
      final restoredTransforms = <String, CortanaModelViewTransform>{};
      if (raw.isNotEmpty) {
        final decoded = jsonDecode(raw);
        if (decoded is List) {
          for (final item in decoded) {
            if (item is! Map) {
              continue;
            }
            final model = CortanaLive2dModelInfo.fromJson(
              Map<String, dynamic>.from(item),
            );
            if (model == null) {
              debugPrint(
                '[Cortana Live2D] skip invalid persisted model: $item',
              );
              continue;
            }
            if (await Live2dModelLocator.isUsableModelJson(
              model.modelJsonPath,
            )) {
              restored.add(model);
            } else {
              debugPrint(
                '[Cortana Live2D] persisted model no longer usable: id=${model.id} name=${model.name} path=${model.modelJsonPath}',
              );
            }
          }
        }
      }
      if (rawTransforms.isNotEmpty) {
        final decodedTransforms = jsonDecode(rawTransforms);
        if (decodedTransforms is Map) {
          for (final entry in decodedTransforms.entries) {
            final modelKey = entry.key.toString().trim();
            final rawValue = entry.value;
            if (modelKey.isEmpty || rawValue is! Map) {
              continue;
            }
            final transform = CortanaModelViewTransform.fromJson(
              Map<String, dynamic>.from(rawValue),
            );
            if (transform != null) {
              restoredTransforms[modelKey] = transform;
            }
          }
        }
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _cortanaLive2dModels = restored;
        _selectedCortanaLive2dModelId =
            restored.any((model) => model.id == selected) ? selected : '';
        _cortanaLive2dViewTransforms = restoredTransforms;
      });
      if (_selectedCortanaLive2dModelId != selected) {
        await prefs.setString(
          _cortanaSelectedLive2dModelKey,
          _selectedCortanaLive2dModelId,
        );
      }
    } catch (err) {
      debugPrint('Restore Cortana Live2D models failed: $err');
    }
  }

  Future<void> _persistCortanaLive2dModels() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      _cortanaLive2dModelsKey,
      jsonEncode(_cortanaLive2dModels.map((model) => model.toJson()).toList()),
    );
    await prefs.setString(
      _cortanaSelectedLive2dModelKey,
      _selectedCortanaLive2dModelId,
    );
  }

  String _cortanaLive2dViewTransformModelKey(String modelId) {
    final id = modelId.trim();
    return id.isEmpty ? 'default' : id;
  }

  CortanaModelViewTransform _selectedCortanaLive2dViewTransform() {
    final modelKey = _cortanaLive2dViewTransformModelKey(
      _selectedCortanaLive2dModelId,
    );
    return _cortanaLive2dViewTransforms[modelKey] ??
        CortanaModelViewTransform.defaults;
  }

  Future<void> _persistCortanaLive2dViewTransforms() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      _cortanaLive2dViewTransformsKey,
      jsonEncode(
        _cortanaLive2dViewTransforms.map(
          (key, value) => MapEntry(key, value.toJson()),
        ),
      ),
    );
  }

  void _handleCortanaLive2dViewTransformChanged(
    CortanaModelViewTransform transform,
  ) {
    final modelKey = _cortanaLive2dViewTransformModelKey(
      _selectedCortanaLive2dModelId,
    );
    final value = transform.normalized();
    setState(() {
      _cortanaLive2dViewTransforms = <String, CortanaModelViewTransform>{
        ..._cortanaLive2dViewTransforms,
        modelKey: value,
      };
    });
    unawaited(_persistCortanaLive2dViewTransforms());
  }

  String _selectedCortanaLive2dModelUrl() {
    final selected = _findCortanaLive2dModel(_selectedCortanaLive2dModelId);
    if (selected == null) {
      if (_selectedCortanaLive2dModelId.isNotEmpty) {
        debugPrint(
          '[Cortana Live2D] selected model id not found: $_selectedCortanaLive2dModelId',
        );
      }
      return '';
    }
    final modelUrl = Uri.file(selected.modelJsonPath).toString();
    debugPrint(
      '[Cortana Live2D] selected model url: id=${selected.id} name=${selected.name} url=$modelUrl',
    );
    return modelUrl;
  }

  CortanaLive2dModelInfo? _findCortanaLive2dModel(String id) {
    for (final model in _cortanaLive2dModels) {
      if (model.id == id) {
        return model;
      }
    }
    return null;
  }

  Uri _normalizeLive2dResourceUri(String raw) {
    final trimmed = raw.trim();
    if (trimmed.isEmpty) {
      throw const FormatException('请输入 Live2D 资源 URL 或本机路径');
    }
    final localFile = File(trimmed);
    if (!trimmed.contains('://') && localFile.existsSync()) {
      return localFile.uri;
    }
    final uri = Uri.parse(trimmed);
    if (!uri.hasScheme) {
      throw const FormatException('资源地址必须是 http(s)、file URL 或有效本机路径');
    }
    final scheme = uri.scheme.toLowerCase();
    if (scheme != 'https' && scheme != 'http' && scheme != 'file') {
      throw const FormatException('Live2D 资源仅支持 http(s)、file URL 或本机路径');
    }
    return uri;
  }

  Future<Uri> _resolveLive2dArchiveUri(Uri uri) async {
    if (uri.scheme.toLowerCase() == 'file') {
      return uri;
    }
    final lowerPath = uri.path.toLowerCase();
    if (lowerPath.endsWith('.zip')) {
      return uri;
    }
    final host = uri.host.toLowerCase();
    if (!host.endsWith('aplaybox.com')) {
      return uri;
    }
    final resp = await http.get(
      uri,
      headers: <String, String>{
        HttpHeaders.userAgentHeader: 'Mozilla/5.0 CortanaLive2DDownloader/1.0',
        HttpHeaders.acceptHeader:
            'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
      },
    );
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw HttpException('APlayBox 页面读取失败: HTTP ${resp.statusCode}');
    }
    final body = utf8.decode(resp.bodyBytes, allowMalformed: true);
    final match = RegExp(
      r"""https?:\\?/\\?/[^"'<>\s]+?\.zip(?:\?[^"'<>\s]*)?""",
      caseSensitive: false,
    ).firstMatch(body);
    if (match == null) {
      throw const FormatException(
        '未在 APlayBox 页面中找到可直接下载的 zip。请登录 APlayBox 后复制实际 zip 下载链接。',
      );
    }
    final url = match.group(0)!.replaceAll(r'\/', '/');
    return Uri.parse(url);
  }

  String _sanitizeLive2dModelName(String raw) {
    final trimmed = raw.trim();
    final fallback = 'live2d_${DateTime.now().millisecondsSinceEpoch}';
    final source = trimmed.isEmpty ? fallback : trimmed;
    return source
        .replaceAll(RegExp(r'[\\/:*?"<>|]+'), '_')
        .replaceAll(RegExp(r'\s+'), '_')
        .replaceAll(RegExp(r'_+'), '_')
        .replaceAll(RegExp(r'^_+|_+$'), '');
  }

  Future<CortanaLive2dModelInfo> _extractCortanaLive2dArchive({
    required File archiveFile,
    required String sourceUrl,
  }) async {
    final root = await _getCortanaLive2dRootDir();
    await root.create(recursive: true);
    final installId = DateTime.now().millisecondsSinceEpoch.toString();
    final tempDir = Directory(
      '${root.path}${Platform.pathSeparator}.$installId.extracting',
    );
    final destDir = Directory(
      '${root.path}${Platform.pathSeparator}$installId',
    );
    await _deleteDirectoryIfExists(tempDir);
    await tempDir.create(recursive: true);
    try {
      final archiveBytes = await archiveFile.readAsBytes();
      debugPrint(
        '[Cortana Live2D] extracting archive: path=${archiveFile.path} bytes=${archiveBytes.length} source=$sourceUrl',
      );
      addFlutterClientLog(
        'Live2D 解压开始: ${archiveFile.path} (${_formatBytes(archiveBytes.length)})',
      );
      final archive = ZipDecoder().decodeBytes(archiveBytes);
      final modelEntries = archive
          .where((entry) => entry.isFile && entry.name.endsWith('.model3.json'))
          .map((entry) => entry.name)
          .toList(growable: false);
      final modelEntriesText = modelEntries.join(', ');
      debugPrint(
        '[Cortana Live2D] archive entries=${archive.length} '
        'modelEntries=$modelEntriesText',
      );
      final modelEntriesSummary = modelEntries.isEmpty
          ? '未找到'
          : modelEntriesText;
      addFlutterClientLog(
        'Live2D 压缩包条目: ${archive.length}, model3=$modelEntriesSummary',
      );
      final canonicalRoot = tempDir.absolute.uri;
      for (final entry in archive) {
        final outUri = canonicalRoot.resolve(entry.name);
        if (!outUri.toFilePath().startsWith(canonicalRoot.toFilePath())) {
          throw FormatException('压缩包包含不安全路径: ${entry.name}');
        }
        final outputPath = outUri.toFilePath();
        if (entry.isFile) {
          final file = File(outputPath);
          await file.parent.create(recursive: true);
          await file.writeAsBytes(entry.content as List<int>);
        } else {
          await Directory(outputPath).create(recursive: true);
        }
      }
      final modelJsonPath = await Live2dModelLocator.findModelJson(
        tempDir.path,
      );
      if (modelJsonPath == null) {
        throw const FormatException('压缩包内未找到可用的 Cubism 3+ .model3.json 资源');
      }
      final normalized = await Live2dModelNormalizer.normalize(modelJsonPath);
      debugPrint('[Cortana Live2D] normalized model: ${normalized.toJson()}');
      addFlutterClientLog(
        'Live2D 规范化完成: 表情=${normalized.expressionCount} 动作=${normalized.motionCount} 贴图=${normalized.textureMaxSize}px/${normalized.textureTotalPixels ~/ (1024 * 1024)}MP',
      );
      for (final warning in normalized.warnings) {
        debugPrint('[Cortana Live2D] normalize warning: $warning');
        addFlutterClientLog('Live2D 兼容提示: $warning');
      }
      await _deleteDirectoryIfExists(destDir);
      await tempDir.rename(destDir.path);
      final finalModelJsonPath = modelJsonPath.replaceFirst(
        tempDir.path,
        destDir.path,
      );
      final modelName = _sanitizeLive2dModelName(
        finalModelJsonPath
            .split(Platform.pathSeparator)
            .last
            .replaceFirst('.model3.json', ''),
      );
      debugPrint(
        '[Cortana Live2D] install prepared: id=$installId name=$modelName root=${destDir.path} model=$finalModelJsonPath',
      );
      addFlutterClientLog('Live2D 模型定位成功: $finalModelJsonPath');
      return CortanaLive2dModelInfo(
        id: installId,
        name: modelName,
        rootPath: destDir.path,
        modelJsonPath: finalModelJsonPath,
        sourceUrl: sourceUrl,
        installedAtMs: DateTime.now().millisecondsSinceEpoch,
        manifestPath: normalized.manifestPath.replaceFirst(
          tempDir.path,
          destDir.path,
        ),
      );
    } catch (err, stackTrace) {
      debugPrint('[Cortana Live2D] extract failed: $err');
      debugPrint('$stackTrace');
      addFlutterClientLog('Live2D 解压失败: $err');
      await _deleteDirectoryIfExists(tempDir);
      await _deleteDirectoryIfExists(destDir);
      rethrow;
    }
  }

  Future<void> _copyLocalLive2dArchive(Uri uri, File archiveFile) async {
    final source = File(uri.toFilePath());
    if (!await source.exists()) {
      throw FileSystemException('Live2D 资源不存在', source.path);
    }
    final length = await source.length();
    debugPrint(
      '[Cortana Live2D] copying local archive: ${source.path} bytes=$length -> ${archiveFile.path}',
    );
    addFlutterClientLog(
      'Live2D 读取本机压缩包: ${source.path} (${_formatBytes(length)})',
    );
    await archiveFile.parent.create(recursive: true);
    await _deleteFileIfExists(archiveFile);
    await source.copy(archiveFile.path);
  }

  Future<CortanaLive2dModelInfo> _installCortanaLive2dArchive({
    required File archiveFile,
    required String sourceUrl,
  }) async {
    final model = await _extractCortanaLive2dArchive(
      archiveFile: archiveFile,
      sourceUrl: sourceUrl,
    );
    await _deleteFileIfExists(archiveFile);
    if (!mounted) {
      return model;
    }
    setState(() {
      _cortanaLive2dModels = <CortanaLive2dModelInfo>[
        model,
        ..._cortanaLive2dModels.where((item) => item.id != model.id),
      ];
      _selectedCortanaLive2dModelId = model.id;
      _cortanaLive2dDownloadProgress = 1.0;
      _status = 'Cortana Live2D 形象已切换: ${model.name}';
    });
    await _persistCortanaLive2dModels();
    final modelUrl = Uri.file(model.modelJsonPath).toString();
    debugPrint(
      '[Cortana Live2D] installed and selected: id=${model.id} name=${model.name} model=${model.modelJsonPath} url=$modelUrl',
    );
    addFlutterClientLog('Live2D 已安装并选中: ${model.name} $modelUrl');
    _appendSystem('Cortana Live2D 形象已安装并切换: ${model.name}');
    return model;
  }

  Future<void> _downloadCortanaLive2dFromResourceUrl() async {
    if (_cortanaLive2dDownloading) {
      return;
    }
    setState(() {
      _cortanaLive2dDownloading = true;
      _cortanaLive2dDownloadProgress = 0.0;
      _cortanaLive2dDownloadError = '';
      _status = '正在准备下载 Cortana Live2D 形象...';
    });
    try {
      final inputUri = _normalizeLive2dResourceUri(_cortanaLive2dUrlCtrl.text);
      final archiveUri = await _resolveLive2dArchiveUri(inputUri);
      final archiveFile = await _getCortanaLive2dArchiveFile();
      debugPrint(
        '[Cortana Live2D] install from input=$inputUri archive=$archiveUri temp=${archiveFile.path}',
      );
      addFlutterClientLog('Live2D 安装来源: $archiveUri');
      if (archiveUri.scheme.toLowerCase() == 'file') {
        await _copyLocalLive2dArchive(archiveUri, archiveFile);
        if (mounted) {
          setState(() {
            _cortanaLive2dDownloadProgress = 1.0;
            _status = '已读取本机 Live2D 资源，正在解压...';
          });
        }
      } else {
        await _fileDownloader.downloadToFile(
          archiveUri,
          destinationPath: archiveFile.path,
          headersBuilder: ({int? rangeStart}) => <String, String>{
            HttpHeaders.userAgentHeader:
                'Mozilla/5.0 CortanaLive2DDownloader/1.0',
            HttpHeaders.refererHeader: inputUri.toString(),
            if (rangeStart != null && rangeStart > 0)
              HttpHeaders.rangeHeader: 'bytes=$rangeStart-',
          },
          onProgress: (receivedBytes, totalBytes, resumed) {
            if (!mounted) {
              return;
            }
            final progress = totalBytes != null && totalBytes > 0
                ? receivedBytes / totalBytes
                : 0.0;
            setState(() {
              _cortanaLive2dDownloadProgress = progress;
              _status = totalBytes != null && totalBytes > 0
                  ? '正在下载 Cortana Live2D... ${_formatBytes(receivedBytes)} / ${_formatBytes(totalBytes)}'
                  : '正在下载 Cortana Live2D... ${_formatBytes(receivedBytes)}';
            });
          },
        );
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _status = '正在解压 Cortana Live2D 资源...';
      });
      await _installCortanaLive2dArchive(
        archiveFile: archiveFile,
        sourceUrl: inputUri.toString(),
      );
    } catch (err) {
      if (!mounted) {
        return;
      }
      setState(() {
        _cortanaLive2dDownloadError = err.toString();
        _status = 'Cortana Live2D 下载失败';
      });
    } finally {
      if (mounted) {
        setState(() => _cortanaLive2dDownloading = false);
      }
    }
  }

  Future<void> _installCortanaLive2dResourceItem(AppResourceItem item) async {
    if (item.fileId.isEmpty || _cortanaLive2dDownloading) {
      return;
    }
    setState(() {
      _cortanaLive2dDownloading = true;
      _cortanaLive2dDownloadProgress = 0.0;
      _cortanaLive2dDownloadError = '';
      _status = '正在从资源库加载 Live2D...';
    });
    try {
      final archiveFile = await _getCortanaLive2dArchiveFile();
      debugPrint(
        '[Cortana Live2D] install resource item: fileId=${item.fileId} name=${item.fileName} size=${item.fileSize} format=${item.fileFormat} provider=${item.storageProvider} objectKey=${item.objectKey}',
      );
      addFlutterClientLog(
        'Live2D 资源库安装: ${item.fileName} fileId=${item.fileId} size=${_formatBytes(item.fileSize)}',
      );
      await _runAuthed('Download Live2D resource', (client) {
        return client.downloadAttachmentToFile(
          item.fileId,
          destinationPath: archiveFile.path,
          attachmentMeta: <String, dynamic>{
            'file_id': item.fileId,
            'file_name': item.fileName,
            'file_size': item.fileSize,
            'file_format': item.fileFormat,
            'storage_provider': item.storageProvider,
            'object_key': item.objectKey,
          },
          onProgress: (receivedBytes, totalBytes, resumed) {
            if (!mounted) {
              return;
            }
            final progress = totalBytes != null && totalBytes > 0
                ? receivedBytes / totalBytes
                : 0.0;
            setState(() {
              _cortanaLive2dDownloadProgress = progress;
              _status = totalBytes != null && totalBytes > 0
                  ? '正在加载 Live2D... ${_formatBytes(receivedBytes)} / ${_formatBytes(totalBytes)}'
                  : '正在加载 Live2D... ${_formatBytes(receivedBytes)}';
            });
          },
        );
      });
      if (!mounted) {
        return;
      }
      setState(() {
        _status = '正在解压 Cortana Live2D 资源...';
      });
      await _installCortanaLive2dArchive(
        archiveFile: archiveFile,
        sourceUrl: item.downloadUrl.isNotEmpty
            ? item.downloadUrl
            : 'app-resource://${item.fileId}',
      );
    } catch (err) {
      if (!mounted) {
        return;
      }
      setState(() {
        _cortanaLive2dDownloadError = err.toString();
        _status = 'Cortana Live2D 资源加载失败';
      });
    } finally {
      if (mounted) {
        setState(() => _cortanaLive2dDownloading = false);
      }
    }
  }

  Future<void> _selectCortanaLive2dModel(String id) async {
    final model = _findCortanaLive2dModel(id);
    final modelName = model?.name ?? 'default';
    final modelPath = model?.modelJsonPath ?? '';
    debugPrint(
      '[Cortana Live2D] select model: id=$id name=$modelName path=$modelPath',
    );
    addFlutterClientLog(
      id.isEmpty
          ? 'Live2D 切换到默认 Haru'
          : 'Live2D 切换到: ${model?.name ?? id} $modelPath',
    );
    setState(() {
      _selectedCortanaLive2dModelId = id;
      _status = id.isEmpty ? '已切回默认 Cortana 形象' : '已切换 Cortana 形象';
    });
    await _persistCortanaLive2dModels();
  }

  Future<void> _deleteCortanaLive2dModel(CortanaLive2dModelInfo model) async {
    await _deleteDirectoryIfExists(Directory(model.rootPath));
    setState(() {
      _cortanaLive2dModels = _cortanaLive2dModels
          .where((item) => item.id != model.id)
          .toList(growable: false);
      _cortanaLive2dViewTransforms =
          Map<String, CortanaModelViewTransform>.from(
            _cortanaLive2dViewTransforms,
          )..remove(_cortanaLive2dViewTransformModelKey(model.id));
      if (_selectedCortanaLive2dModelId == model.id) {
        _selectedCortanaLive2dModelId = '';
      }
      _status = '已删除 Cortana Live2D 形象: ${model.name}';
    });
    await _persistCortanaLive2dModels();
    await _persistCortanaLive2dViewTransforms();
  }

  Future<String?> _resolveAvailableVoskModelPath({
    String? preferredPath,
  }) async {
    final localModelPath = await _getLocalVoskModelPath();
    final candidatePaths = <String>[
      if (preferredPath != null && preferredPath.trim().isNotEmpty)
        preferredPath.trim(),
      localModelPath,
    ];

    String? lastCandidate;
    for (final candidatePath in candidatePaths) {
      if (candidatePath == lastCandidate) {
        continue;
      }
      lastCandidate = candidatePath;
      final resolvedPath = await VoskModelLocator.findModelRoot(candidatePath);
      if (resolvedPath != null) {
        return resolvedPath;
      }
    }

    return null;
  }

  Future<bool> _isVoskModelDownloaded() async {
    try {
      final modelPath = await _resolveAvailableVoskModelPath();
      return modelPath != null && await VoskModelLocator.isModelRoot(modelPath);
    } catch (_) {
      return false;
    }
  }

  Future<bool> _hasPartialVoskDownload() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await _migrateLegacyVoskPartialArchive(prefs);
      final partFile = await _getVoskArchivePartFile();
      return await partFile.exists() && await partFile.length() > 0;
    } catch (_) {
      return false;
    }
  }

  Future<double> _getVoskDownloadProgress() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      return prefs.getDouble(_voskDownloadProgressKey) ?? 0.0;
    } catch (_) {
      return 0.0;
    }
  }

  Future<void> _downloadAndExtractVoskModel() async {
    if (_voskModelDownloading) {
      return;
    }

    final prefs = await SharedPreferences.getInstance();
    final modelPath = await _getLocalVoskModelPath();
    final archiveFile = await _getVoskArchiveFile();
    final partFile = await _getVoskArchivePartFile();
    final tempModelDir = await _getVoskExtractionTempDir();

    await _migrateLegacyVoskPartialArchive(prefs);
    final savedProgress = await _getVoskDownloadProgress();
    final savedBytes = prefs.getInt(_voskDownloadBytesKey) ?? 0;

    setState(() {
      _voskModelDownloading = true;
      _voskModelDownloadProgress = savedProgress;
      _voskModelDownloadError = null;
      if (savedBytes > 0) {
        _status = '继续下载 Vosk 语音模型... 已下载 ${_formatBytes(savedBytes)}';
      } else {
        _status = '正在下载 Vosk 语音模型...';
      }
    });

    var extractionStarted = false;
    try {
      final hasPendingPart = await partFile.exists();
      final hasCompleteArchive = await archiveFile.exists() && !hasPendingPart;
      if (hasCompleteArchive) {
        if (!mounted) {
          return;
        }
        setState(() {
          _status = '检测到已下载完成的 Vosk 压缩包，正在继续解压...';
          _voskModelDownloadProgress = 1.0;
        });
      } else {
        await _fileDownloader.downloadToFile(
          Uri.parse(_voskModelUrl),
          destinationPath: archiveFile.path,
          headersBuilder: ({int? rangeStart}) => <String, String>{
            if (rangeStart != null && rangeStart > 0)
              HttpHeaders.rangeHeader: 'bytes=$rangeStart-',
          },
          onProgress: (receivedBytes, totalBytes, resumed) {
            final progress = totalBytes != null && totalBytes > 0
                ? receivedBytes / totalBytes
                : 0.0;
            if (!mounted) {
              return;
            }
            setState(() {
              _voskModelDownloadProgress = progress;
              if (totalBytes != null && totalBytes > 0) {
                _status =
                    '正在下载 Vosk 语音模型... ${_formatBytes(receivedBytes)} / ${_formatBytes(totalBytes)} (${(progress * 100).toStringAsFixed(1)}%)';
              } else {
                _status = '正在下载 Vosk 语音模型... ${_formatBytes(receivedBytes)}';
              }
            });
            unawaited(prefs.setInt(_voskDownloadBytesKey, receivedBytes));
            if (totalBytes != null && totalBytes > 0) {
              unawaited(prefs.setDouble(_voskDownloadProgressKey, progress));
            }
          },
          onRetry: (error, attempt, delay) {
            if (!mounted) {
              return;
            }
            setState(() {
              _status =
                  '下载中断，${delay.inSeconds} 秒后重试 ($attempt/${_voskDownloadRetryDelays.length})...';
            });
          },
        );
        final archiveBytes = await archiveFile.length();
        await prefs.setInt(_voskDownloadBytesKey, archiveBytes);
        await prefs.setDouble(_voskDownloadProgressKey, 1.0);
      }

      if (!mounted) {
        return;
      }

      setState(() {
        _status = '正在解压 Vosk 语音模型...';
      });

      extractionStarted = true;
      await _deleteDirectoryIfExists(tempModelDir);

      String? resolvedModelPath;
      if (_isAndroidHost) {
        final extractResp = await _zipExtractor.extractZip(
          archiveFile.path,
          modelPath,
        );
        final extractedModelPath = (extractResp['modelPath'] ?? '')
            .toString()
            .trim();
        if (extractedModelPath.isNotEmpty) {
          resolvedModelPath = extractedModelPath;
        }
      } else {
        final bytes = await archiveFile.readAsBytes();
        final archive = ZipDecoder().decodeBytes(bytes);
        await tempModelDir.create(recursive: true);
        for (final file in archive) {
          final filePath =
              '${tempModelDir.path}${Platform.pathSeparator}${file.name}';
          if (file.isFile) {
            final outputFile = File(filePath);
            await outputFile.create(recursive: true);
            await outputFile.writeAsBytes(file.content as List<int>);
          } else {
            await Directory(filePath).create(recursive: true);
          }
        }
        final extractedTempRoot = await _resolveAvailableVoskModelPath(
          preferredPath: tempModelDir.path,
        );
        if (extractedTempRoot == null) {
          throw const FormatException(
            'Extracted Vosk model is incomplete. Missing required files.',
          );
        }
        await _deleteDirectoryIfExists(Directory(modelPath));
        await tempModelDir.rename(modelPath);
        resolvedModelPath = await _resolveAvailableVoskModelPath(
          preferredPath: modelPath,
        );
      }

      resolvedModelPath ??= await _resolveAvailableVoskModelPath(
        preferredPath: modelPath,
      );
      if (resolvedModelPath == null) {
        throw const FormatException(
          'Extracted Vosk model is incomplete. Missing required files.',
        );
      }

      await _deleteFileIfExists(archiveFile);
      await prefs.remove(_voskDownloadProgressKey);
      await prefs.remove(_voskDownloadBytesKey);

      if (!mounted) {
        return;
      }

      setState(() {
        _voskModelDownloadProgress = 1.0;
        _status = 'Vosk 语音模型下载完成';
      });

      await prefs.setString('vosk_model_path', resolvedModelPath);

      _appendSystem('Vosk 语音模型已下载完成，正在初始化...');

      await _loadClientConfig();
    } catch (err, stack) {
      if (extractionStarted) {
        await prefs.remove('vosk_model_path');
        await prefs.remove(_voskDownloadProgressKey);
        await prefs.remove(_voskDownloadBytesKey);
        await _deleteFileIfExists(archiveFile);
        await _deleteFileIfExists(partFile);
        await _deleteDirectoryIfExists(tempModelDir);
        final currentModelRoot = await _resolveAvailableVoskModelPath(
          preferredPath: modelPath,
        );
        if (currentModelRoot == null) {
          await _deleteDirectoryIfExists(Directory(modelPath));
        }
      }
      if (!mounted) {
        return;
      }
      debugPrint('Download Vosk model error: $err\n$stack');
      setState(() {
        _voskModelDownloadError = err.toString();
        _status = 'Vosk 模型下载失败: $err';
      });
      _appendSystem('Vosk 模型下载失败: $err。点击下载按钮可继续下载。');
    } finally {
      if (mounted) {
        setState(() {
          _voskModelDownloading = false;
        });
      }
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
      final savedUserId = prefs.getString(_lastLoginUserIdKey)?.trim() ?? '';
      // Use saved model path if available (from downloaded model), otherwise use asset config
      final savedModelPath = prefs.getString('vosk_model_path')?.trim() ?? '';
      final config = ClientConfig(
        baseUrl: savedBaseUrl.isEmpty ? assetConfig.baseUrl : savedBaseUrl,
        receiveToken: assetConfig.receiveToken,
        enableLocalVosk: assetConfig.enableLocalVosk,
        voskModelPath: savedModelPath.isNotEmpty
            ? savedModelPath
            : assetConfig.voskModelPath,
        cortanaEnabledDefault: assetConfig.cortanaEnabledDefault,
        cortanaAllowFullAccessDefault:
            assetConfig.cortanaAllowFullAccessDefault,
        cortanaAutoPlayDefault: assetConfig.cortanaAutoPlayDefault,
        cortanaProactiveModeDefault: assetConfig.cortanaProactiveModeDefault,
        cortanaHighFreqStartHourDefault:
            assetConfig.cortanaHighFreqStartHourDefault,
        cortanaHighFreqStartMinuteDefault:
            assetConfig.cortanaHighFreqStartMinuteDefault,
        cortanaHighFreqEndHourDefault:
            assetConfig.cortanaHighFreqEndHourDefault,
        cortanaHighFreqEndMinuteDefault:
            assetConfig.cortanaHighFreqEndMinuteDefault,
        cortanaPersonaNameDefault: assetConfig.cortanaPersonaNameDefault,
        cortanaPersonaDescriptionDefault:
            assetConfig.cortanaPersonaDescriptionDefault,
      );
      if (config.baseUrl.isEmpty) {
        throw const FormatException('base_url is required');
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _clientConfig = config;
        _cortanaEnabled =
            prefs.getBool(_cortanaEnabledKey) ?? config.cortanaEnabledDefault;
        _cortanaAllowFullAccess =
            prefs.getBool(_cortanaAllowFullAccessKey) ??
            config.cortanaAllowFullAccessDefault;
        _cortanaAutoPlay =
            prefs.getBool(_cortanaAutoPlayKey) ?? config.cortanaAutoPlayDefault;
        _cortanaProactiveMode =
            prefs.getString(_cortanaProactiveModeKey)?.trim().isNotEmpty == true
            ? prefs.getString(_cortanaProactiveModeKey)!.trim()
            : config.cortanaProactiveModeDefault;
        _cortanaHighFreqStartHour =
            prefs.getInt(_cortanaHighFreqStartHourKey) ??
            config.cortanaHighFreqStartHourDefault;
        _cortanaHighFreqStartMinute =
            prefs.getInt(_cortanaHighFreqStartMinuteKey) ??
            config.cortanaHighFreqStartMinuteDefault;
        _cortanaHighFreqEndHour =
            prefs.getInt(_cortanaHighFreqEndHourKey) ??
            config.cortanaHighFreqEndHourDefault;
        _cortanaHighFreqEndMinute =
            prefs.getInt(_cortanaHighFreqEndMinuteKey) ??
            config.cortanaHighFreqEndMinuteDefault;
        _cortanaPersonaName =
            prefs.getString(_cortanaPersonaNameKey)?.trim().isNotEmpty == true
            ? prefs.getString(_cortanaPersonaNameKey)!.trim()
            : config.cortanaPersonaNameDefault;
        _cortanaOwnerTitle =
            prefs.getString(_cortanaOwnerTitleKey)?.trim() ??
            _defaultCortanaOwnerTitle;
        _cortanaPersonaDescription =
            prefs.getString(_cortanaPersonaDescriptionKey)?.trim().isNotEmpty ==
                true
            ? prefs.getString(_cortanaPersonaDescriptionKey)!.trim()
            : config.cortanaPersonaDescriptionDefault;
        _cortanaVoiceWakeEnabled =
            prefs.getBool(_cortanaVoiceWakeEnabledKey) ??
            _defaultCortanaVoiceWakeEnabled;
        _cortanaWakePhrase =
            prefs.getString(_cortanaWakePhraseKey)?.trim().isNotEmpty == true
            ? prefs.getString(_cortanaWakePhraseKey)!.trim()
            : _defaultCortanaWakePhrase;
        _cortanaPersonaNameCtrl.text = _cortanaPersonaName;
        _cortanaOwnerTitleCtrl.text = _cortanaOwnerTitle;
        _cortanaPersonaDescCtrl.text = _cortanaPersonaDescription;
        _cortanaWakePhraseCtrl.text = _cortanaWakePhrase;
        _baseUrlController.text = config.baseUrl;
        if (savedUserId.isNotEmpty) {
          _userIdController.text = savedUserId;
        }
        _configLoading = false;
        _configError = '';
        _status = 'Config loaded';
      });
      _appendSystem('Client config loaded.');
      _maybeShowStartupGreeting();
      unawaited(_initVoice());
      unawaited(_restoreSavedLogin());
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
        enableLocalVosk: _clientConfig?.enableLocalVosk ?? false,
        voskModelPath: _clientConfig?.voskModelPath ?? '',
        cortanaEnabledDefault: _clientConfig?.cortanaEnabledDefault ?? true,
        cortanaAllowFullAccessDefault:
            _clientConfig?.cortanaAllowFullAccessDefault ?? true,
        cortanaAutoPlayDefault: _clientConfig?.cortanaAutoPlayDefault ?? true,
        cortanaProactiveModeDefault:
            _clientConfig?.cortanaProactiveModeDefault ?? 'high',
        cortanaHighFreqStartHourDefault:
            _clientConfig?.cortanaHighFreqStartHourDefault ?? 9,
        cortanaHighFreqStartMinuteDefault:
            _clientConfig?.cortanaHighFreqStartMinuteDefault ?? 0,
        cortanaHighFreqEndHourDefault:
            _clientConfig?.cortanaHighFreqEndHourDefault ?? 22,
        cortanaHighFreqEndMinuteDefault:
            _clientConfig?.cortanaHighFreqEndMinuteDefault ?? 0,
        cortanaPersonaNameDefault:
            _clientConfig?.cortanaPersonaNameDefault ?? 'Cortana',
        cortanaPersonaDescriptionDefault:
            _clientConfig?.cortanaPersonaDescriptionDefault ?? '',
      );
      _status = 'URL updated';
    });
    _appendSystem('Server URL updated: $baseUrl');
  }

  CortanaSettings get _cortanaSettings => CortanaSettings(
    enabled: _cortanaEnabled,
    allowFullAccess: _cortanaAllowFullAccess,
    autoPlay: _cortanaAutoPlay,
    proactiveMode: _cortanaProactiveMode,
    highFreqStartHour: _cortanaHighFreqStartHour,
    highFreqStartMinute: _cortanaHighFreqStartMinute,
    highFreqEndHour: _cortanaHighFreqEndHour,
    highFreqEndMinute: _cortanaHighFreqEndMinute,
    personaName: _cortanaPersonaName,
    ownerTitle: _cortanaOwnerTitle,
    personaDescription: _cortanaPersonaDescription,
    voiceWakeEnabled: _cortanaVoiceWakeEnabled,
    wakePhrase: _cortanaWakePhrase,
  );

  Future<void> _syncCortanaSettings({bool silent = false}) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_cortanaEnabledKey, _cortanaEnabled);
    await prefs.setBool(_cortanaAllowFullAccessKey, _cortanaAllowFullAccess);
    await prefs.setBool(_cortanaAutoPlayKey, _cortanaAutoPlay);
    await prefs.setString(_cortanaProactiveModeKey, _cortanaProactiveMode);
    await prefs.setInt(_cortanaHighFreqStartHourKey, _cortanaHighFreqStartHour);
    await prefs.setInt(
      _cortanaHighFreqStartMinuteKey,
      _cortanaHighFreqStartMinute,
    );
    await prefs.setInt(_cortanaHighFreqEndHourKey, _cortanaHighFreqEndHour);
    await prefs.setInt(_cortanaHighFreqEndMinuteKey, _cortanaHighFreqEndMinute);
    await prefs.setString(_cortanaPersonaNameKey, _cortanaPersonaName);
    await prefs.setString(_cortanaOwnerTitleKey, _cortanaOwnerTitle);
    await prefs.setString(
      _cortanaPersonaDescriptionKey,
      _cortanaPersonaDescription,
    );
    await prefs.setBool(_cortanaVoiceWakeEnabledKey, _cortanaVoiceWakeEnabled);
    await prefs.setString(_cortanaWakePhraseKey, _cortanaWakePhrase);
    if (_clientConfig == null ||
        (_sessionToken.isEmpty && _refreshToken.isEmpty) ||
        _userIdController.text.trim().isEmpty) {
      return;
    }
    final payload = _cortanaSettings.toJson();
    try {
      await _runAuthed(
        'Sync Cortana settings',
        (client) => client.saveCortanaSettings(payload),
      );
      if (!silent) {
        _appendSystem('Cortana 设置已同步');
      }
    } catch (err) {
      if (!silent) {
        _appendSystem(
          _describeRequestError(err, operation: 'Sync Cortana settings'),
        );
      }
    }
  }

  Map<String, dynamic>? _currentCortanaDeviceContext() {
    final context = _lastCortanaDeviceContext;
    if (context == null || context.isEmpty) {
      final live2d = _currentCortanaLive2dContext();
      return live2d == null ? null : <String, dynamic>{'live2d': live2d};
    }
    final copy = Map<String, dynamic>.from(context);
    final live2d = _currentCortanaLive2dContext();
    if (live2d != null) {
      copy['live2d'] = live2d;
    }
    return copy;
  }

  Map<String, dynamic>? _currentCortanaLive2dContext() {
    final selected = _findCortanaLive2dModel(_selectedCortanaLive2dModelId);
    if (selected == null) {
      return null;
    }
    final live2d = <String, dynamic>{
      'model_id': selected.id,
      'model_name': selected.name,
    };
    final manifestPath = selected.manifestPath.trim().isNotEmpty
        ? selected.manifestPath.trim()
        : '${selected.rootPath}${Platform.pathSeparator}cortana_manifest.json';
    try {
      final manifestFile = File(manifestPath);
      if (manifestFile.existsSync()) {
        final decoded = jsonDecode(
          manifestFile.readAsStringSync(encoding: utf8),
        );
        if (decoded is Map) {
          final manifest = Map<String, dynamic>.from(decoded);
          final expressions = manifest['expressions'];
          final motions = manifest['motions'];
          if (expressions is Map && expressions.isNotEmpty) {
            live2d['available_expressions'] = expressions.keys
                .map((key) => key.toString())
                .where((key) => key.trim().isNotEmpty)
                .toList();
          }
          if (motions is Map && motions.isNotEmpty) {
            live2d['available_motions'] = motions.map((group, rawItems) {
              final items = rawItems is List
                  ? rawItems
                        .whereType<Map>()
                        .map(
                          (item) =>
                              (item['File'] ?? item['file'] ?? '').toString(),
                        )
                        .where((path) => path.trim().isNotEmpty)
                        .toList()
                  : <String>[];
              return MapEntry(group.toString(), items);
            });
          }
        }
      }
    } catch (err) {
      debugPrint('[Cortana Live2D] read manifest failed: $err');
    }
    return live2d;
  }

  Future<Map<String, dynamic>?> _refreshCortanaDeviceContext({
    bool report = false,
    bool force = false,
  }) async {
    if (_cortanaLocationUpdating) {
      return _currentCortanaDeviceContext();
    }
    if (!force && (!_cortanaEnabled || !_cortanaAllowFullAccess)) {
      return _currentCortanaDeviceContext();
    }
    if (_clientConfig == null ||
        (_sessionToken.isEmpty && _refreshToken.isEmpty) ||
        _userIdController.text.trim().isEmpty) {
      return _currentCortanaDeviceContext();
    }

    _cortanaLocationUpdating = true;
    try {
      final location = _isAndroidHost
          ? await _locationProvider.getCurrentLocation()
          : <String, dynamic>{
              'available': false,
              'permission': 'unsupported_platform',
              'timestamp': DateTime.now().millisecondsSinceEpoch,
            };
      final locationLog = _describeCortanaLocationForLog(location);
      debugPrint('[Cortana Device Context] location captured: $locationLog');
      addFlutterClientLog('定位采集: $locationLog');
      final now = DateTime.now();
      final context = <String, dynamic>{
        'client': <String, dynamic>{
          'platform': kIsWeb ? 'web' : Platform.operatingSystem,
          'app_version': appVersion,
          'captured_at': now.millisecondsSinceEpoch,
          'timezone': now.timeZoneName,
          'timezone_offset_minutes': now.timeZoneOffset.inMinutes,
        },
        'location': location,
      };
      final live2d = _currentCortanaLive2dContext();
      if (live2d != null) {
        context['live2d'] = live2d;
      }
      _lastCortanaDeviceContext = context;
      debugPrint('[Cortana Device Context] payload: ${jsonEncode(context)}');

      final shouldReport =
          report &&
          (force ||
              _lastCortanaLocationReportAt == null ||
              now.difference(_lastCortanaLocationReportAt!) >
                  const Duration(minutes: 10));
      if (shouldReport) {
        _lastCortanaLocationReportAt = now;
        unawaited(
          _runAuthed('Report Cortana device context', (client) {
                return client.sendCortanaEvent(
                  'device_context_update',
                  meta: <String, dynamic>{
                    'summary': _buildCortanaLocationSummary(location),
                    'device_context': context,
                  },
                );
              })
              .then((_) {
                debugPrint(
                  '[Cortana Device Context] report sent: $locationLog',
                );
                addFlutterClientLog('定位上报成功: $locationLog');
              })
              .catchError((Object err, StackTrace _) {
                debugPrint('[Cortana Device Context] report failed: $err');
                addFlutterClientLog('定位上报失败: $err');
              }),
        );
      }
      return context;
    } catch (err) {
      debugPrint('[Cortana Device Context] refresh failed: $err');
      return _currentCortanaDeviceContext();
    } finally {
      _cortanaLocationUpdating = false;
    }
  }

  String _buildCortanaLocationSummary(Map<String, dynamic> location) {
    if (location['available'] == true) {
      final lat = location['latitude'];
      final lon = location['longitude'];
      final accuracy = location['accuracy_m'];
      return '客户端位置已更新: lat=$lat lon=$lon accuracy_m=$accuracy';
    }
    final permission = (location['permission'] ?? '').toString().trim();
    final message = (location['message'] ?? '').toString().trim();
    return [
      '客户端位置不可用',
      if (permission.isNotEmpty) 'permission=$permission',
      if (message.isNotEmpty) message,
    ].join(' ');
  }

  String _describeCortanaLocationForLog(Map<String, dynamic> location) {
    if (location['available'] == true) {
      final lat = location['latitude'];
      final lon = location['longitude'];
      final accuracy = location['accuracy_m'];
      final provider = (location['provider'] ?? '').toString().trim();
      final locationTime = location['location_time'];
      final message = (location['message'] ?? '').toString().trim();
      return [
        'available=true',
        'lat=$lat',
        'lon=$lon',
        'accuracy_m=$accuracy',
        if (provider.isNotEmpty) 'provider=$provider',
        if (locationTime != null) 'location_time=$locationTime',
        if (message.isNotEmpty) 'message=$message',
      ].join(' ');
    }
    final permission = (location['permission'] ?? '').toString().trim();
    final message = (location['message'] ?? '').toString().trim();
    final providerEnabled = location['provider_enabled'];
    return [
      'available=false',
      if (permission.isNotEmpty) 'permission=$permission',
      if (providerEnabled != null) 'provider_enabled=$providerEnabled',
      if (message.isNotEmpty) 'message=$message',
    ].join(' ');
  }

  void _scheduleCortanaLocationRefresh({required bool initial}) {
    _cortanaLocationTimer?.cancel();
    final delay = initial ? const Duration(seconds: 2) : Duration.zero;
    Future<void>.delayed(delay, () {
      if (!mounted) {
        return;
      }
      unawaited(_refreshCortanaDeviceContext(report: true));
    });
    _cortanaLocationTimer = Timer.periodic(const Duration(minutes: 15), (_) {
      if (!mounted) {
        return;
      }
      unawaited(_refreshCortanaDeviceContext(report: true));
    });
  }

  void _applyCortanaSettings(CortanaSettings settings) {
    setState(() {
      _cortanaEnabled = settings.enabled;
      _cortanaAllowFullAccess = settings.allowFullAccess;
      _cortanaAutoPlay = settings.autoPlay;
      _cortanaProactiveMode = settings.proactiveMode;
      _cortanaHighFreqStartHour = settings.highFreqStartHour;
      _cortanaHighFreqStartMinute = settings.highFreqStartMinute;
      _cortanaHighFreqEndHour = settings.highFreqEndHour;
      _cortanaHighFreqEndMinute = settings.highFreqEndMinute;
      _cortanaPersonaName = settings.personaName;
      _cortanaOwnerTitle = settings.ownerTitle;
      _cortanaPersonaDescription = settings.personaDescription;
      _cortanaVoiceWakeEnabled = settings.voiceWakeEnabled;
      _cortanaWakePhrase = settings.wakePhrase.trim().isEmpty
          ? _defaultCortanaWakePhrase
          : settings.wakePhrase.trim();
    });
    if (_cortanaPersonaNameCtrl.text != _cortanaPersonaName) {
      _cortanaPersonaNameCtrl.text = _cortanaPersonaName;
    }
    if (_cortanaOwnerTitleCtrl.text != _cortanaOwnerTitle) {
      _cortanaOwnerTitleCtrl.text = _cortanaOwnerTitle;
    }
    if (_cortanaPersonaDescCtrl.text != _cortanaPersonaDescription) {
      _cortanaPersonaDescCtrl.text = _cortanaPersonaDescription;
    }
    if (_cortanaWakePhraseCtrl.text != _cortanaWakePhrase) {
      _cortanaWakePhraseCtrl.text = _cortanaWakePhrase;
    }
    if (_cortanaVoiceWakeEnabled && _cortanaEnabled) {
      _scheduleCortanaWakeRestart();
    } else {
      unawaited(_pauseCortanaWakeListening(cancel: true));
    }
    if (_cortanaEnabled && _cortanaAllowFullAccess) {
      unawaited(_refreshCortanaDeviceContext(report: true, force: true));
    }
    unawaited(_syncCortanaSettings());
  }

  bool get _sessionExpired {
    if (_sessionToken.isEmpty) {
      return true;
    }
    if (_sessionExpiresAtMs <= 0) {
      return false;
    }
    return DateTime.now().millisecondsSinceEpoch >= _sessionExpiresAtMs;
  }

  bool get _sessionNeedsRefresh {
    if (_refreshToken.trim().isEmpty) {
      return false;
    }
    if (_sessionToken.isEmpty) {
      return true;
    }
    if (_sessionExpiresAtMs <= 0) {
      return false;
    }
    return DateTime.now().millisecondsSinceEpoch >=
        _sessionExpiresAtMs - _sessionRefreshSkew.inMilliseconds;
  }

  Future<void> _persistRefreshToken({
    required String userId,
    required String refreshToken,
  }) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_lastLoginUserIdKey, userId.trim());
      if (refreshToken.trim().isNotEmpty) {
        await _secureStorage.write(
          key: _refreshTokenStorageKey,
          value: refreshToken.trim(),
        );
      }
    } catch (err) {
      debugPrint('Persist refresh token failed: $err');
    }
  }

  Future<String> _readStoredRefreshToken() async {
    try {
      return (await _secureStorage.read(key: _refreshTokenStorageKey) ?? '')
          .trim();
    } catch (err) {
      debugPrint('Read refresh token failed: $err');
      return '';
    }
  }

  Future<void> _clearStoredRefreshToken() async {
    try {
      await _secureStorage.delete(key: _refreshTokenStorageKey);
    } catch (err) {
      debugPrint('Clear refresh token failed: $err');
    }
  }

  Future<void> _closeSocketForSessionReset() async {
    _reconnectTimer?.cancel();
    await _socketSub?.cancel();
    await _socket?.sink.close();
    _socketSub = null;
    _socket = null;
    _connecting = false;
    _connected = false;
    _autoReconnect = false;
  }

  Future<void> _applySessionTokens(
    AppAuthSession session, {
    String? statusText,
  }) async {
    final nextRefreshToken = session.refreshToken.trim().isEmpty
        ? _refreshToken.trim()
        : session.refreshToken.trim();
    _userIdController.text = session.userId;
    if (mounted) {
      setState(() {
        _sessionToken = session.accessToken;
        _refreshToken = nextRefreshToken;
        _sessionExpiresAtMs = session.expiresAtMs;
        if (session.obsAgentBaseUrl.isNotEmpty) {
          _obsAgentBaseUrl = session.obsAgentBaseUrl;
        }
        if (statusText != null && statusText.trim().isNotEmpty) {
          _status = statusText.trim();
        }
      });
    } else {
      _sessionToken = session.accessToken;
      _refreshToken = nextRefreshToken;
      _sessionExpiresAtMs = session.expiresAtMs;
      if (session.obsAgentBaseUrl.isNotEmpty) {
        _obsAgentBaseUrl = session.obsAgentBaseUrl;
      }
    }
    await _persistRefreshToken(
      userId: session.userId,
      refreshToken: nextRefreshToken,
    );
  }

  Future<void> _finishAuthenticatedLogin(
    AppAuthSession session, {
    required String successStatus,
    bool clearPassword = true,
    String? successMessage,
  }) async {
    await _closeSocketForSessionReset();
    await _applySessionTokens(session, statusText: successStatus);
    if (clearPassword) {
      _passwordController.clear();
    }
    _historyPersistence.invalidate();
    if (mounted) {
      setState(() {
        _lastSequence = 0;
        _currentGroupId = '';
        _historyByScope.clear();
        _loadedHistoryScopes.clear();
        _seenMessageIds.clear();
        _codegenStreamStates.clear();
        _pendingCodegenStreamIds.clear();
        _activeCodegenHistoryId = '';
        _consumedCortanaReplyKeys.clear();
        _autoInstallTriggered.clear();
        _groups.clear();
      });
    } else {
      _lastSequence = 0;
      _currentGroupId = '';
      _historyByScope.clear();
      _loadedHistoryScopes.clear();
      _seenMessageIds.clear();
      _codegenStreamStates.clear();
      _pendingCodegenStreamIds.clear();
      _activeCodegenHistoryId = '';
      _consumedCortanaReplyKeys.clear();
      _autoInstallTriggered.clear();
      _groups.clear();
    }
    await _loadAllHistoryForUser();
    await _refreshGroups();
    await _loadCodegenProjects(silent: true);
    await _syncCortanaSettings(silent: true);
    await _refreshCortanaDeviceContext(report: true, force: true);
    if (successMessage != null && successMessage.trim().isNotEmpty) {
      _appendSystem(successMessage.trim());
    }
    _maybeShowLoginGreeting(restored: !clearPassword);
    unawaited(_connectWs());
    unawaited(_restoreVoskDownloadProgress());
  }

  Future<void> _clearLocalAuthState({
    required String status,
    bool clearStoredRefreshToken = true,
  }) async {
    if (clearStoredRefreshToken) {
      await _clearStoredRefreshToken();
    }
    await _closeSocketForSessionReset();
    _historyPersistence.invalidate();
    if (mounted) {
      setState(() {
        _loggingIn = false;
        _sessionToken = '';
        _refreshToken = '';
        _sessionExpiresAtMs = 0;
        _obsAgentBaseUrl = '';
        _lastSequence = 0;
        _currentGroupId = '';
        _historyByScope.clear();
        _loadedHistoryScopes.clear();
        _seenMessageIds.clear();
        _codegenStreamStates.clear();
        _pendingCodegenStreamIds.clear();
        _activeCodegenHistoryId = '';
        _consumedCortanaReplyKeys.clear();
        _autoInstallTriggered.clear();
        _groups.clear();
        _codingProjects.clear();
        _deployProjects.clear();
        _codegenError = '';
        _status = status;
        _loginGreetingShown = false;
      });
    } else {
      _loggingIn = false;
      _sessionToken = '';
      _refreshToken = '';
      _sessionExpiresAtMs = 0;
      _obsAgentBaseUrl = '';
      _lastSequence = 0;
      _currentGroupId = '';
      _historyByScope.clear();
      _loadedHistoryScopes.clear();
      _seenMessageIds.clear();
      _codegenStreamStates.clear();
      _pendingCodegenStreamIds.clear();
      _activeCodegenHistoryId = '';
      _consumedCortanaReplyKeys.clear();
      _autoInstallTriggered.clear();
      _groups.clear();
      _codingProjects.clear();
      _deployProjects.clear();
      _codegenError = '';
      _status = status;
      _loginGreetingShown = false;
    }
  }

  bool _isUnauthorizedError(Object err) {
    if (err is AppAgentUnauthorizedException) {
      return true;
    }
    final text = err.toString().toLowerCase();
    return text.contains('401') ||
        text.contains('unauthorized') ||
        text.contains('login required');
  }

  Future<bool> _refreshSessionTokens({required bool forceRefresh}) async {
    final inFlight = _sessionRefreshFuture;
    if (inFlight != null) {
      return inFlight;
    }

    final future = () async {
      final userId = _userIdController.text.trim();
      final refreshToken = _refreshToken.trim();
      if (_clientConfig == null || userId.isEmpty || refreshToken.isEmpty) {
        return _sessionToken.isNotEmpty && !forceRefresh;
      }
      try {
        final session = await _client.refreshSession(refreshToken);
        await _applySessionTokens(session);
        return true;
      } on AppAgentUnauthorizedException {
        await _clearLocalAuthState(status: 'Login expired');
        _appendSystem('登录已过期，请重新输入密码登录。');
        return false;
      } catch (err) {
        if (!forceRefresh && _sessionToken.isNotEmpty && !_sessionExpired) {
          return true;
        }
        rethrow;
      }
    }();

    _sessionRefreshFuture = future;
    try {
      return await future;
    } finally {
      if (identical(_sessionRefreshFuture, future)) {
        _sessionRefreshFuture = null;
      }
    }
  }

  Future<bool> _ensureSessionReady({bool forceRefresh = false}) async {
    if (!forceRefresh && _sessionToken.isNotEmpty && !_sessionNeedsRefresh) {
      return true;
    }
    if (_refreshToken.trim().isEmpty) {
      return _sessionToken.isNotEmpty && !forceRefresh;
    }
    return _refreshSessionTokens(forceRefresh: forceRefresh);
  }

  Future<void> _restoreSavedLogin() async {
    if (_clientConfig == null || _loggingIn || _sessionToken.isNotEmpty) {
      return;
    }
    final refreshToken = await _readStoredRefreshToken();
    if (refreshToken.isEmpty) {
      return;
    }
    if (mounted) {
      setState(() {
        _loggingIn = true;
        _refreshToken = refreshToken;
        _status = 'Restoring login...';
      });
    } else {
      _loggingIn = true;
      _refreshToken = refreshToken;
      _status = 'Restoring login...';
    }
    try {
      final refreshed = await _ensureSessionReady(forceRefresh: true);
      if (!refreshed || _sessionToken.isEmpty) {
        return;
      }
      final session = AppAuthSession(
        userId: _userIdController.text.trim(),
        accessToken: _sessionToken,
        refreshToken: _refreshToken,
        expiresAtMs: _sessionExpiresAtMs,
        obsAgentBaseUrl: _obsAgentBaseUrl,
      );
      await _finishAuthenticatedLogin(
        session,
        successStatus: 'Login restored, connecting WebSocket...',
        clearPassword: false,
        successMessage: '已从安全存储恢复登录。',
      );
    } catch (err) {
      if (mounted) {
        setState(() {
          _status = 'Auto login failed';
        });
      } else {
        _status = 'Auto login failed';
      }
      _appendSystem(_describeRequestError(err, operation: 'Restore login'));
    } finally {
      if (mounted) {
        setState(() {
          _loggingIn = false;
        });
      } else {
        _loggingIn = false;
      }
    }
  }
}
