part of 'main.dart';

class VoskTranscriber {
  static const MethodChannel _channel = MethodChannel(
    'com.example.flutter_client_for_appagent/vosk',
  );

  Future<String?> initialize(String modelPath) async {
    final resp = await _channel.invokeMapMethod<String, dynamic>('initialize', {
      'modelPath': modelPath,
    });
    if (resp == null) {
      return 'Vosk initialize returned empty response';
    }
    final ready = resp['ready'] == true;
    final message = (resp['message'] ?? '').toString().trim();
    if (ready) {
      return null;
    }
    return message.isEmpty ? 'Vosk initialize failed' : message;
  }

  Future<String> transcribeFile(String audioPath) async {
    final resp = await _channel.invokeMapMethod<String, dynamic>(
      'transcribeFile',
      {'audioPath': audioPath},
    );
    return (resp?['text'] ?? '').toString().trim();
  }

  void setWakeWordEventHandler(
    Future<void> Function(Map<String, dynamic> event)? handler,
  ) {
    _channel.setMethodCallHandler((call) async {
      if (call.method != 'wakeWordEvent' || handler == null) {
        return;
      }
      final args = call.arguments;
      if (args is Map) {
        AppDebugRecorder.instance.recordVoskNativeEvent(
          Map<String, dynamic>.from(args),
        );
        try {
          await handler(Map<String, dynamic>.from(args));
        } catch (err, stack) {
          addFlutterClientLog('Vosk wakeWordEvent handler failed: $err');
          AppDebugRecorder.instance.recordMethodChannelError(
            _channel.name,
            call.method,
            err,
            stack,
          );
          debugPrint('Vosk wakeWordEvent handler failed: $err\n$stack');
        }
      }
    });
  }

  Future<bool> startWakeWordListening() async {
    final resp = await _channel.invokeMapMethod<String, dynamic>(
      'startWakeWordListening',
    );
    return resp?['started'] == true;
  }

  Future<void> stopWakeWordListening() async {
    await _channel.invokeMethod<void>('stopWakeWordListening');
  }

  Future<String> readNativeDebugTrace(String category) async {
    final resp = await _channel.invokeMapMethod<String, dynamic>(
      'readNativeDebugTrace',
      {'category': category},
    );
    return (resp?['content'] ?? '').toString();
  }
}

class ApkInstaller {
  static const MethodChannel _channel = MethodChannel(
    'com.example.flutter_client_for_appagent/installer',
  );

  Future<Map<String, dynamic>> installApk(String apkPath) async {
    final resp = await _channel.invokeMapMethod<String, dynamic>('installApk', {
      'apkPath': apkPath,
    });
    return resp == null ? <String, dynamic>{} : Map<String, dynamic>.from(resp);
  }
}

class ZipExtractor {
  static const MethodChannel _channel = MethodChannel(
    'com.example.flutter_client_for_appagent/zip',
  );

  Future<Map<String, dynamic>> extractZip(
    String zipPath,
    String destPath,
  ) async {
    final resp = await _channel.invokeMapMethod<String, dynamic>('extractZip', {
      'zipPath': zipPath,
      'destPath': destPath,
    });
    if (resp == null) {
      throw Exception('Zip extraction returned null');
    }
    final success = resp['success'] == true;
    final error = (resp['error'] ?? '').toString().trim();
    if (!success) {
      throw Exception(error.isEmpty ? 'Zip extraction failed' : error);
    }
    return resp;
  }
}

class LocalFilePicker {
  static const MethodChannel _channel = MethodChannel(
    'com.example.flutter_client_for_appagent/file_picker',
  );

  Future<String?> pickFile() async {
    final resp = await _channel.invokeMapMethod<String, dynamic>('pickFile');
    if (resp == null) {
      return null;
    }
    final path = (resp['path'] ?? '').toString().trim();
    return path.isEmpty ? null : path;
  }
}

class CortanaLive2dModelInfo {
  const CortanaLive2dModelInfo({
    required this.id,
    required this.name,
    required this.rootPath,
    required this.modelJsonPath,
    required this.sourceUrl,
    required this.installedAtMs,
    this.manifestPath = '',
  });

  final String id;
  final String name;
  final String rootPath;
  final String modelJsonPath;
  final String sourceUrl;
  final int installedAtMs;
  final String manifestPath;

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'id': id,
      'name': name,
      'root_path': rootPath,
      'model_json_path': modelJsonPath,
      'source_url': sourceUrl,
      'installed_at_ms': installedAtMs,
      if (manifestPath.isNotEmpty) 'manifest_path': manifestPath,
    };
  }

  static CortanaLive2dModelInfo? fromJson(Map<String, dynamic> json) {
    final id = (json['id'] ?? '').toString().trim();
    final name = (json['name'] ?? '').toString().trim();
    final rootPath = (json['root_path'] ?? '').toString().trim();
    final modelJsonPath = (json['model_json_path'] ?? '').toString().trim();
    if (id.isEmpty ||
        name.isEmpty ||
        rootPath.isEmpty ||
        modelJsonPath.isEmpty) {
      return null;
    }
    final installedAtRaw = json['installed_at_ms'];
    return CortanaLive2dModelInfo(
      id: id,
      name: name,
      rootPath: rootPath,
      modelJsonPath: modelJsonPath,
      sourceUrl: (json['source_url'] ?? '').toString().trim(),
      installedAtMs: installedAtRaw is int
          ? installedAtRaw
          : int.tryParse('$installedAtRaw') ?? 0,
      manifestPath: (json['manifest_path'] ?? '').toString().trim(),
    );
  }
}

class Live2dModelLocator {
  const Live2dModelLocator._();

  static Future<String?> findModelJson(String rootPath) async {
    final root = Directory(rootPath);
    if (!await root.exists()) {
      debugPrint('[Cortana Live2D] model root missing: $rootPath');
      return null;
    }
    var candidateCount = 0;
    await for (final entity in root.list(recursive: true, followLinks: false)) {
      if (entity is! File || !entity.path.endsWith('.model3.json')) {
        continue;
      }
      candidateCount++;
      debugPrint('[Cortana Live2D] checking model candidate: ${entity.path}');
      if (await isUsableModelJson(entity.path)) {
        debugPrint('[Cortana Live2D] usable model selected: ${entity.path}');
        return entity.path;
      }
      debugPrint('[Cortana Live2D] unusable model candidate: ${entity.path}');
    }
    debugPrint(
      '[Cortana Live2D] no usable .model3.json under $rootPath, candidates=$candidateCount',
    );
    return null;
  }

  static Future<bool> isUsableModelJson(String path) async {
    try {
      final file = File(path);
      if (!await file.exists()) {
        debugPrint('[Cortana Live2D] model json missing: $path');
        return false;
      }
      final decoded = jsonDecode(await file.readAsString(encoding: utf8));
      if (decoded is! Map) {
        debugPrint('[Cortana Live2D] model json is not an object: $path');
        return false;
      }
      final refs = decoded['FileReferences'];
      if (refs is! Map) {
        debugPrint('[Cortana Live2D] FileReferences missing: $path');
        return false;
      }
      final moc = (refs['Moc'] ?? '').toString().trim();
      final textures = refs['Textures'];
      if (moc.isEmpty || textures is! List || textures.isEmpty) {
        final textureSummary = textures is List ? textures.length : 'invalid';
        debugPrint(
          '[Cortana Live2D] required references missing: '
          '$path moc=$moc textures=$textureSummary',
        );
        return false;
      }
      final baseDir = file.parent.path;
      if (!await File('$baseDir${Platform.pathSeparator}$moc').exists()) {
        debugPrint(
          '[Cortana Live2D] moc file missing: $baseDir${Platform.pathSeparator}$moc',
        );
        return false;
      }
      for (final texture in textures) {
        final texturePath = texture.toString().trim();
        if (texturePath.isEmpty) {
          debugPrint('[Cortana Live2D] empty texture reference in: $path');
          return false;
        }
        if (!await File(
          '$baseDir${Platform.pathSeparator}$texturePath',
        ).exists()) {
          debugPrint(
            '[Cortana Live2D] texture file missing: $baseDir${Platform.pathSeparator}$texturePath',
          );
          return false;
        }
      }
      return true;
    } catch (err, stackTrace) {
      debugPrint('[Cortana Live2D] model json check failed: $path error=$err');
      debugPrint('$stackTrace');
      return false;
    }
  }
}

class Live2dModelNormalizationResult {
  const Live2dModelNormalizationResult({
    required this.modelJsonPath,
    required this.manifestPath,
    required this.expressionCount,
    required this.motionCount,
    required this.textureMaxSize,
    required this.textureTotalPixels,
    required this.warnings,
  });

  final String modelJsonPath;
  final String manifestPath;
  final int expressionCount;
  final int motionCount;
  final int textureMaxSize;
  final int textureTotalPixels;
  final List<String> warnings;

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'model_json_path': modelJsonPath,
      'manifest_path': manifestPath,
      'expression_count': expressionCount,
      'motion_count': motionCount,
      'texture_max_size': textureMaxSize,
      'texture_total_pixels': textureTotalPixels,
      'warnings': warnings,
    };
  }
}

class Live2dModelNormalizer {
  const Live2dModelNormalizer._();

  static const int webTextureWarningSize = 4096;
  static const int hardTextureLimit = 8192;
  static const int textureTotalWarningPixels = 64 * 1024 * 1024;
  static const int textureTotalHardPixels = 96 * 1024 * 1024;
  static const int webTextureFallbackSize = 2048;

  static Future<Live2dModelNormalizationResult> normalize(
    String modelJsonPath,
  ) async {
    final modelFile = File(modelJsonPath);
    if (!await modelFile.exists()) {
      throw const FormatException('zip包格式错误：未找到 Live2D model3.json');
    }

    final decoded = jsonDecode(await modelFile.readAsString(encoding: utf8));
    if (decoded is! Map) {
      throw const FormatException('zip包格式错误：model3.json 不是合法对象');
    }

    final modelJson = Map<String, dynamic>.from(decoded);
    final refsRaw = modelJson['FileReferences'];
    final refs = refsRaw is Map
        ? Map<String, dynamic>.from(refsRaw)
        : <String, dynamic>{};
    if (refs.isEmpty) {
      throw const FormatException('zip包格式错误：model3.json 缺少 FileReferences');
    }

    final baseDir = modelFile.parent;
    final moc = (refs['Moc'] ?? '').toString().trim();
    if (moc.isEmpty) {
      throw const FormatException('zip包格式错误：模型缺少 moc 引用');
    }
    if (!await _referencedFileExists(baseDir, moc)) {
      throw FormatException('zip包格式错误：模型缺少 moc 文件 $moc');
    }

    final textures = refs['Textures'];
    if (textures is! List || textures.isEmpty) {
      throw const FormatException('zip包格式错误：模型缺少贴图引用');
    }

    final textureWarnings = <String>[];
    var textureInfos = <({File file, String path, int width, int height})>[];
    for (final texture in textures) {
      final texturePath = texture.toString().trim();
      if (texturePath.isEmpty) {
        throw const FormatException('zip包格式错误：模型包含空贴图路径');
      }
      final textureFile = _resolveReferencedFile(baseDir, texturePath);
      if (!await textureFile.exists()) {
        throw FormatException('zip包格式错误：模型缺少贴图文件 $texturePath');
      }
      final size = await _readPngSize(textureFile);
      if (size != null) {
        textureInfos.add((
          file: textureFile,
          path: texturePath,
          width: size.width,
          height: size.height,
        ));
      }
    }

    var textureMaxSize = _maxTextureSide(textureInfos);
    var textureTotalPixels = _totalTexturePixels(textureInfos);
    final shouldGenerateFallback =
        textureMaxSize > webTextureWarningSize ||
        textureTotalPixels > textureTotalHardPixels;
    if (shouldGenerateFallback) {
      textureWarnings.add(
        '原始贴图过大：${textures.length} 张 / ${textureTotalPixels ~/ (1024 * 1024)}MP，已生成 ${webTextureFallbackSize}px WebView 兼容版',
      );
      textureInfos = await _downscaleTexturesForWebView(
        textureInfos,
        webTextureFallbackSize,
      );
      textureMaxSize = _maxTextureSide(textureInfos);
      textureTotalPixels = _totalTexturePixels(textureInfos);
    }

    if (textureTotalPixels > textureTotalWarningPixels) {
      textureWarnings.add(
        '贴图总量较大：${textures.length} 张 / ${textureTotalPixels ~/ (1024 * 1024)}MP，低端设备可能无法渲染',
      );
    }
    for (final info in textureInfos) {
      if (math.max(info.width, info.height) >= webTextureWarningSize) {
        textureWarnings.add(
          '贴图 ${_relativePath(baseDir, info.file)} 尺寸 ${info.width}x${info.height}，WebView/Chrome 可能不支持',
        );
      }
    }

    _fillSingleReference(refs, baseDir, modelFile, 'Physics', '.physics3.json');
    _fillSingleReference(refs, baseDir, modelFile, 'DisplayInfo', '.cdi3.json');
    _fillSingleReference(refs, baseDir, modelFile, 'Pose', '.pose3.json');

    final expressionRefs = await _normalizeExpressions(refs, baseDir);
    final motionRefs = await _normalizeMotions(refs, baseDir);
    _normalizeGroups(modelJson);

    modelJson['FileReferences'] = refs;
    const encoder = JsonEncoder.withIndent('  ');
    await modelFile.writeAsString(
      '${encoder.convert(modelJson)}\n',
      encoding: utf8,
    );

    final warnings = <String>[
      ...textureWarnings,
      if (expressionRefs.isEmpty) '未发现表情文件，Cortana 表情会降级',
      if (motionRefs.isEmpty) '未发现动作文件，Cortana 动作会降级',
    ];
    final manifestFile = File(
      '${baseDir.path}${Platform.pathSeparator}cortana_manifest.json',
    );
    final manifest = <String, dynamic>{
      'version': 1,
      'model': _relativePath(baseDir, modelFile),
      'capabilities': <String, dynamic>{
        'has_expressions': expressionRefs.isNotEmpty,
        'has_motions': motionRefs.isNotEmpty,
        'has_lip_sync': _groupHasIds(modelJson, 'LipSync'),
        'has_eye_blink': _groupHasIds(modelJson, 'EyeBlink'),
        'texture_max_size': textureMaxSize,
        'texture_total_pixels': textureTotalPixels,
      },
      'expressions': <String, dynamic>{
        for (final item in expressionRefs)
          (item['Name'] ?? '').toString(): (item['File'] ?? '').toString(),
      },
      'motions': motionRefs,
      'warnings': warnings,
    };
    await manifestFile.writeAsString(
      '${encoder.convert(manifest)}\n',
      encoding: utf8,
    );

    return Live2dModelNormalizationResult(
      modelJsonPath: modelFile.path,
      manifestPath: manifestFile.path,
      expressionCount: expressionRefs.length,
      motionCount: motionRefs.values.fold<int>(
        0,
        (sum, items) => sum + items.length,
      ),
      textureMaxSize: textureMaxSize,
      textureTotalPixels: textureTotalPixels,
      warnings: warnings,
    );
  }

  static Future<List<Map<String, String>>> _normalizeExpressions(
    Map<String, dynamic> refs,
    Directory baseDir,
  ) async {
    final existing = refs['Expressions'];
    final expressions = existing is List
        ? existing.whereType<Map>().map((item) {
            return item.map<String, String>(
              (key, value) => MapEntry('$key', '$value'),
            );
          }).toList()
        : <Map<String, String>>[];
    final seen = expressions
        .map((item) => (item['File'] ?? '').toString())
        .where((path) => path.isNotEmpty)
        .toSet();
    final files = await _listFilesBySuffix(baseDir, '.exp3.json');
    for (final file in files) {
      final relative = _relativePath(baseDir, file);
      if (!seen.add(relative)) {
        continue;
      }
      final name = _referenceName(relative, '.exp3.json');
      expressions.add(<String, String>{
        'Name': name.isEmpty ? 'expr_${expressions.length + 1}' : name,
        'File': relative,
      });
    }
    if (expressions.isNotEmpty) {
      refs['Expressions'] = expressions;
    }
    return expressions;
  }

  static Future<Map<String, List<Map<String, String>>>> _normalizeMotions(
    Map<String, dynamic> refs,
    Directory baseDir,
  ) async {
    final existing = refs['Motions'];
    final Map<String, List<Map<String, String>>> motions = existing is Map
        ? existing.map<String, List<Map<String, String>>>((key, value) {
            final items = value is List
                ? value
                      .whereType<Map>()
                      .map(
                        (item) => item.map<String, String>(
                          (itemKey, itemValue) =>
                              MapEntry('$itemKey', '$itemValue'),
                        ),
                      )
                      .toList()
                : <Map<String, String>>[];
            return MapEntry('$key', items);
          })
        : <String, List<Map<String, String>>>{};
    final seen = <String>{};
    for (final items in motions.values) {
      for (final item in items) {
        final path = (item['File'] ?? '').toString();
        if (path.isNotEmpty) {
          seen.add(path);
        }
      }
    }
    final files = await _listFilesBySuffix(baseDir, '.motion3.json');
    for (final file in files) {
      final relative = _relativePath(baseDir, file);
      if (!seen.add(relative)) {
        continue;
      }
      final group = _motionGroupForPath(relative);
      motions.putIfAbsent(group, () => <Map<String, String>>[]).add(
        <String, String>{'File': relative},
      );
    }
    if (motions.isNotEmpty) {
      refs['Motions'] = motions;
    }
    return motions;
  }

  static void _normalizeGroups(Map<String, dynamic> modelJson) {
    final groupsRaw = modelJson['Groups'];
    final groups = groupsRaw is List
        ? groupsRaw
              .whereType<Map>()
              .map((item) => Map<String, dynamic>.from(item))
              .toList()
        : <Map<String, dynamic>>[];

    void upsertGroup(String name, List<String> ids) {
      for (final group in groups) {
        if ((group['Name'] ?? '').toString() == name) {
          final existingIds = group['Ids'];
          if (existingIds is! List || existingIds.isEmpty) {
            group['Ids'] = ids;
          }
          group['Target'] = 'Parameter';
          return;
        }
      }
      groups.add(<String, dynamic>{
        'Target': 'Parameter',
        'Name': name,
        'Ids': ids,
      });
    }

    upsertGroup('EyeBlink', <String>['ParamEyeLOpen', 'ParamEyeROpen']);
    upsertGroup('LipSync', <String>['ParamMouthOpenY']);
    modelJson['Groups'] = groups;
  }

  static bool _groupHasIds(Map<String, dynamic> modelJson, String name) {
    final groups = modelJson['Groups'];
    if (groups is! List) {
      return false;
    }
    for (final group in groups) {
      if (group is! Map || (group['Name'] ?? '').toString() != name) {
        continue;
      }
      final ids = group['Ids'];
      return ids is List && ids.isNotEmpty;
    }
    return false;
  }

  static void _fillSingleReference(
    Map<String, dynamic> refs,
    Directory baseDir,
    File modelFile,
    String key,
    String suffix,
  ) {
    if ((refs[key] ?? '').toString().trim().isNotEmpty) {
      return;
    }
    final modelBase = cortanaPathFileName(
      modelFile.path,
    ).replaceFirst('.model3.json', '');
    final candidates =
        Directory(baseDir.path)
            .listSync(recursive: false, followLinks: false)
            .whereType<File>()
            .where((file) => file.path.toLowerCase().endsWith(suffix))
            .toList()
          ..sort((a, b) {
            final aName = cortanaPathFileName(a.path);
            final bName = cortanaPathFileName(b.path);
            final aPreferred = aName.startsWith(modelBase) ? 0 : 1;
            final bPreferred = bName.startsWith(modelBase) ? 0 : 1;
            return aPreferred == bPreferred
                ? aName.compareTo(bName)
                : aPreferred.compareTo(bPreferred);
          });
    if (candidates.isNotEmpty) {
      refs[key] = _relativePath(baseDir, candidates.first);
    }
  }

  static int _maxTextureSide(
    List<({File file, String path, int width, int height})> textures,
  ) {
    var maxSide = 0;
    for (final texture in textures) {
      maxSide = math.max(maxSide, math.max(texture.width, texture.height));
    }
    return maxSide;
  }

  static int _totalTexturePixels(
    List<({File file, String path, int width, int height})> textures,
  ) {
    var total = 0;
    for (final texture in textures) {
      total += texture.width * texture.height;
    }
    return total;
  }

  static Future<List<({File file, String path, int width, int height})>>
  _downscaleTexturesForWebView(
    List<({File file, String path, int width, int height})> textures,
    int maxSide,
  ) async {
    final out = <({File file, String path, int width, int height})>[];
    for (final texture in textures) {
      final sourceMaxSide = math.max(texture.width, texture.height);
      if (sourceMaxSide <= maxSide) {
        out.add(texture);
        continue;
      }
      final scale = maxSide / sourceMaxSide;
      final targetWidth = math.max(1, (texture.width * scale).round());
      final targetHeight = math.max(1, (texture.height * scale).round());
      try {
        final codec = await instantiateImageCodec(
          await texture.file.readAsBytes(),
          targetWidth: targetWidth,
          targetHeight: targetHeight,
        );
        final frame = await codec.getNextFrame();
        final byteData = await frame.image.toByteData(
          format: ImageByteFormat.png,
        );
        frame.image.dispose();
        codec.dispose();
        if (byteData == null) {
          throw StateError('PNG encoder returned null');
        }
        await texture.file.writeAsBytes(byteData.buffer.asUint8List());
        out.add((
          file: texture.file,
          path: texture.path,
          width: targetWidth,
          height: targetHeight,
        ));
      } catch (error) {
        throw FormatException(
          'zip包格式错误：贴图无法生成 WebView 兼容版 ${texture.path}: $error',
        );
      }
    }
    return out;
  }

  static Future<List<File>> _listFilesBySuffix(
    Directory baseDir,
    String suffix,
  ) async {
    final files = <File>[];
    await for (final entity in baseDir.list(
      recursive: true,
      followLinks: false,
    )) {
      if (entity is File && entity.path.toLowerCase().endsWith(suffix)) {
        files.add(entity);
      }
    }
    files.sort(
      (a, b) => _relativePath(baseDir, a).compareTo(_relativePath(baseDir, b)),
    );
    return files;
  }

  static String _motionGroupForPath(String path) {
    final lower = path.toLowerCase();
    if (lower.contains('tap') || lower.contains('touch')) {
      return 'Tap';
    }
    if (lower.contains('wave') || lower.contains('greet')) {
      return 'IdleWave';
    }
    return 'Idle';
  }

  static String _referenceName(String path, String suffix) {
    final name = cortanaPathFileName(path);
    final lower = name.toLowerCase();
    if (!lower.endsWith(suffix)) {
      return name;
    }
    return name.substring(0, name.length - suffix.length);
  }

  static Future<bool> _referencedFileExists(
    Directory baseDir,
    String relativePath,
  ) async {
    return _resolveReferencedFile(baseDir, relativePath).exists();
  }

  static File _resolveReferencedFile(Directory baseDir, String relativePath) {
    return File(baseDir.uri.resolve(relativePath).toFilePath());
  }

  static String _relativePath(Directory baseDir, File file) {
    final baseUri = baseDir.absolute.uri;
    final fileUri = file.absolute.uri;
    final baseSegments = baseUri.pathSegments;
    final fileSegments = fileUri.pathSegments;
    var index = 0;
    while (index < baseSegments.length &&
        index < fileSegments.length &&
        baseSegments[index] == fileSegments[index]) {
      index++;
    }
    final relativeSegments = <String>[
      for (var i = index; i < baseSegments.length; i++)
        if (baseSegments[i].isNotEmpty) '..',
      ...fileSegments.skip(index),
    ];
    return Uri(pathSegments: relativeSegments).toString();
  }

  static Future<({int width, int height})?> _readPngSize(File file) async {
    final stream = file.openRead(0, 24);
    final bytes = <int>[];
    await for (final chunk in stream) {
      bytes.addAll(chunk);
    }
    if (bytes.length < 24) {
      return null;
    }
    const signature = <int>[137, 80, 78, 71, 13, 10, 26, 10];
    for (var i = 0; i < signature.length; i++) {
      if (bytes[i] != signature[i]) {
        return null;
      }
    }
    int readUint32(int offset) {
      return (bytes[offset] << 24) |
          (bytes[offset + 1] << 16) |
          (bytes[offset + 2] << 8) |
          bytes[offset + 3];
    }

    return (width: readUint32(16), height: readUint32(20));
  }
}

String cortanaPathFileName(String path) {
  final normalized = path.replaceAll('\\', '/');
  final index = normalized.lastIndexOf('/');
  return index < 0 ? normalized : normalized.substring(index + 1);
}

class DeviceLocationProvider {
  static const MethodChannel _channel = MethodChannel(
    'com.example.flutter_client_for_appagent/location',
  );

  Future<Map<String, dynamic>> getCurrentLocation() async {
    debugPrint('[Cortana Location] invoke native getCurrentLocation');
    final resp = await _channel.invokeMapMethod<String, dynamic>(
      'getCurrentLocation',
    );
    if (resp == null) {
      debugPrint('[Cortana Location] native returned null response');
      return <String, dynamic>{
        'available': false,
        'permission': 'unknown',
        'message': 'Location provider returned empty response',
        'timestamp': DateTime.now().millisecondsSinceEpoch,
      };
    }
    final location = Map<String, dynamic>.from(resp);
    debugPrint('[Cortana Location] native response: ${jsonEncode(location)}');
    return location;
  }
}

typedef DownloadProgressCallback =
    void Function(int receivedBytes, int? totalBytes, bool resumed);
typedef DownloadHeadersBuilder =
    Map<String, String> Function({int? rangeStart});
typedef DownloadRetryCallback =
    void Function(Object error, int attempt, Duration delay);

class ResumableFileDownloader {
  const ResumableFileDownloader({
    this.retryDelays = const <Duration>[
      Duration(milliseconds: 300),
      Duration(milliseconds: 800),
      Duration(milliseconds: 1500),
    ],
  });

  final List<Duration> retryDelays;

  Future<void> downloadToFile(
    Uri uri, {
    required String destinationPath,
    required DownloadHeadersBuilder headersBuilder,
    DownloadProgressCallback? onProgress,
    DownloadRetryCallback? onRetry,
  }) async {
    final targetFile = File(destinationPath);
    await targetFile.parent.create(recursive: true);
    final partFile = File('$destinationPath.part');
    var retryCount = 0;

    while (true) {
      final existingBytes = await partFile.exists()
          ? await partFile.length()
          : 0;
      final resumed = existingBytes > 0;
      final client = http.Client();
      IOSink? sink;
      try {
        final request = http.Request('GET', uri);
        request.headers.addAll(
          headersBuilder(rangeStart: resumed ? existingBytes : null),
        );

        final response = await client.send(request);
        if (response.statusCode == HttpStatus.requestedRangeNotSatisfiable &&
            resumed) {
          await _deleteFileIfExists(partFile);
          retryCount = 0;
          continue;
        }
        if (response.statusCode == HttpStatus.unauthorized) {
          final body = await response.stream.bytesToString();
          throw AppAgentUnauthorizedException(
            'download failed: ${response.statusCode} $body',
          );
        }
        if (response.statusCode < 200 || response.statusCode >= 300) {
          final body = await response.stream.bytesToString();
          throw HttpException('download failed: ${response.statusCode} $body');
        }

        if (resumed && response.statusCode != HttpStatus.partialContent) {
          await _deleteFileIfExists(partFile);
          retryCount = 0;
          continue;
        }

        sink = partFile.openWrite(
          mode: resumed ? FileMode.append : FileMode.writeOnly,
        );
        final totalBytes = response.contentLength == null
            ? null
            : resumed
            ? existingBytes + response.contentLength!
            : response.contentLength!;
        var receivedBytes = existingBytes;
        onProgress?.call(receivedBytes, totalBytes, resumed);

        await for (final chunk in response.stream) {
          sink.add(chunk);
          receivedBytes += chunk.length;
          onProgress?.call(receivedBytes, totalBytes, resumed);
        }
        await sink.flush();
        await sink.close();
        sink = null;

        final actualBytes = await partFile.length();
        if (totalBytes != null && actualBytes != totalBytes) {
          throw http.ClientException(
            'download stream ended before completion '
            '(expected $totalBytes bytes, got $actualBytes)',
            uri,
          );
        }

        await _deleteFileIfExists(targetFile);
        await partFile.rename(targetFile.path);
        return;
      } catch (err) {
        if (!_isRecoverableDownloadError(err) ||
            retryCount >= retryDelays.length) {
          rethrow;
        }
        final delay = retryDelays[retryCount];
        retryCount++;
        onRetry?.call(err, retryCount, delay);
        await Future.delayed(delay);
      } finally {
        await sink?.close();
        client.close();
      }
    }
  }

  bool _isRecoverableDownloadError(Object err) {
    return err is SocketException ||
        err is TimeoutException ||
        err is http.ClientException;
  }

  static Future<void> _deleteFileIfExists(File file) async {
    if (await file.exists()) {
      await file.delete();
    }
  }
}

class AppResourceItem {
  const AppResourceItem({
    required this.category,
    required this.fileId,
    required this.fileName,
    required this.fileSize,
    required this.fileFormat,
    required this.mimeType,
    required this.storageProvider,
    required this.objectKey,
    required this.downloadUrl,
    required this.updatedAt,
  });

  factory AppResourceItem.fromJson(Map<String, dynamic> json) {
    final updatedAtValue = json['updated_at'];
    final updatedAtMs = updatedAtValue is int
        ? updatedAtValue
        : int.tryParse('$updatedAtValue') ?? 0;
    final sizeValue = json['file_size'];
    final fileSize = sizeValue is int
        ? sizeValue
        : int.tryParse('$sizeValue') ?? 0;
    return AppResourceItem(
      category: (json['category'] ?? '').toString().trim(),
      fileId: (json['file_id'] ?? '').toString().trim(),
      fileName: (json['file_name'] ?? '').toString().trim(),
      fileSize: fileSize,
      fileFormat: (json['file_format'] ?? '').toString().trim(),
      mimeType: (json['mime_type'] ?? '').toString().trim(),
      storageProvider: (json['storage_provider'] ?? '').toString().trim(),
      objectKey: (json['object_key'] ?? '').toString().trim(),
      downloadUrl: (json['download_url'] ?? '').toString().trim(),
      updatedAt: updatedAtMs > 0
          ? DateTime.fromMillisecondsSinceEpoch(updatedAtMs)
          : DateTime.fromMillisecondsSinceEpoch(0),
    );
  }

  final String category;
  final String fileId;
  final String fileName;
  final int fileSize;
  final String fileFormat;
  final String mimeType;
  final String storageProvider;
  final String objectKey;
  final String downloadUrl;
  final DateTime updatedAt;
}

class AppResourceUsage {
  const AppResourceUsage({
    required this.totalSize,
    required this.totalCount,
    required this.categorySize,
    required this.categoryCount,
  });

  factory AppResourceUsage.fromJson(Map<String, dynamic>? json) {
    int readInt(String key) {
      final value = json?[key];
      return value is int ? value : int.tryParse('$value') ?? 0;
    }

    return AppResourceUsage(
      totalSize: readInt('total_size'),
      totalCount: readInt('total_count'),
      categorySize: readInt('category_size'),
      categoryCount: readInt('category_count'),
    );
  }

  final int totalSize;
  final int totalCount;
  final int categorySize;
  final int categoryCount;
}

class AppResourceListResult {
  const AppResourceListResult({required this.items, required this.usage});

  final List<AppResourceItem> items;
  final AppResourceUsage usage;
}
