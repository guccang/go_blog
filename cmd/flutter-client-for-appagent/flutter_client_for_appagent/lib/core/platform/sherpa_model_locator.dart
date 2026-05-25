import 'dart:io';

enum SherpaModelKind { asr, kws }

class SherpaAsrModelFiles {
  const SherpaAsrModelFiles({
    required this.rootPath,
    required this.modelPath,
    required this.tokensPath,
  });

  final String rootPath;
  final String modelPath;
  final String tokensPath;
}

class SherpaKwsModelFiles {
  const SherpaKwsModelFiles({
    required this.rootPath,
    required this.encoderPath,
    required this.decoderPath,
    required this.joinerPath,
    required this.tokensPath,
  });

  final String rootPath;
  final String encoderPath;
  final String decoderPath;
  final String joinerPath;
  final String tokensPath;
}

class SherpaModelBundle {
  const SherpaModelBundle({required this.asr, required this.kws});

  final SherpaAsrModelFiles asr;
  final SherpaKwsModelFiles kws;
}

class SherpaModelLocator {
  const SherpaModelLocator._();

  static Future<String?> findModelRoot(
    String rootPath,
    SherpaModelKind kind,
  ) async {
    final trimmedPath = rootPath.trim();
    if (trimmedPath.isEmpty) {
      return null;
    }

    final rootDir = Directory(trimmedPath);
    if (!await rootDir.exists()) {
      return null;
    }

    final directRoot = await _matchModelRoot(rootDir, kind);
    if (directRoot != null) {
      return directRoot;
    }

    try {
      await for (final entity in rootDir.list(followLinks: false)) {
        if (entity is! Directory) {
          continue;
        }
        final nestedRoot = await _matchModelRoot(entity, kind);
        if (nestedRoot != null) {
          return nestedRoot;
        }
      }
    } catch (_) {
      return null;
    }

    return null;
  }

  static Future<bool> isModelRoot(String rootPath, SherpaModelKind kind) async {
    final trimmedPath = rootPath.trim();
    if (trimmedPath.isEmpty) {
      return false;
    }
    return _hasRequiredFiles(Directory(trimmedPath), kind);
  }

  static Future<SherpaAsrModelFiles?> findAsrModel(String rootPath) async {
    final root = await findModelRoot(rootPath, SherpaModelKind.asr);
    if (root == null) {
      return null;
    }
    return SherpaAsrModelFiles(
      rootPath: root,
      modelPath: _join(root, 'model.int8.onnx'),
      tokensPath: _join(root, 'tokens.txt'),
    );
  }

  static Future<SherpaKwsModelFiles?> findKwsModel(String rootPath) async {
    final root = await findModelRoot(rootPath, SherpaModelKind.kws);
    if (root == null) {
      return null;
    }
    final encoder = await _firstExistingFile(root, const <String>[
      'encoder-epoch-12-avg-2-chunk-16-left-64.int8.onnx',
      'encoder-epoch-12-avg-2-chunk-16-left-64.onnx',
    ]);
    final decoder = await _firstExistingFile(root, const <String>[
      'decoder-epoch-12-avg-2-chunk-16-left-64.onnx',
    ]);
    final joiner = await _firstExistingFile(root, const <String>[
      'joiner-epoch-12-avg-2-chunk-16-left-64.int8.onnx',
      'joiner-epoch-12-avg-2-chunk-16-left-64.onnx',
    ]);
    if (encoder == null || decoder == null || joiner == null) {
      return null;
    }
    return SherpaKwsModelFiles(
      rootPath: root,
      encoderPath: encoder,
      decoderPath: decoder,
      joinerPath: joiner,
      tokensPath: _join(root, 'tokens.txt'),
    );
  }

  static Future<SherpaModelBundle?> findModelBundle({
    required String asrRootPath,
    required String kwsRootPath,
  }) async {
    final asr = await findAsrModel(asrRootPath);
    final kws = await findKwsModel(kwsRootPath);
    if (asr == null || kws == null) {
      return null;
    }
    return SherpaModelBundle(asr: asr, kws: kws);
  }

  static Future<String?> _matchModelRoot(
    Directory dir,
    SherpaModelKind kind,
  ) async {
    if (await _hasRequiredFiles(dir, kind)) {
      return dir.path;
    }
    return null;
  }

  static Future<bool> _hasRequiredFiles(
    Directory dir,
    SherpaModelKind kind,
  ) async {
    if (kind == SherpaModelKind.asr) {
      return await _isFile(_join(dir.path, 'model.int8.onnx')) &&
          await _isFile(_join(dir.path, 'tokens.txt'));
    }

    return await _firstExistingFile(dir.path, const <String>[
              'encoder-epoch-12-avg-2-chunk-16-left-64.int8.onnx',
              'encoder-epoch-12-avg-2-chunk-16-left-64.onnx',
            ]) !=
            null &&
        await _firstExistingFile(dir.path, const <String>[
              'decoder-epoch-12-avg-2-chunk-16-left-64.onnx',
            ]) !=
            null &&
        await _firstExistingFile(dir.path, const <String>[
              'joiner-epoch-12-avg-2-chunk-16-left-64.int8.onnx',
              'joiner-epoch-12-avg-2-chunk-16-left-64.onnx',
            ]) !=
            null &&
        await _isFile(_join(dir.path, 'tokens.txt'));
  }

  static Future<String?> _firstExistingFile(
    String rootPath,
    List<String> relativePaths,
  ) async {
    for (final relativePath in relativePaths) {
      final path = _join(rootPath, relativePath);
      if (await _isFile(path)) {
        return path;
      }
    }
    return null;
  }

  static Future<bool> _isFile(String path) async {
    final entityType = await FileSystemEntity.type(path, followLinks: false);
    return entityType == FileSystemEntityType.file;
  }

  static String _join(String left, String right) {
    return '$left${Platform.pathSeparator}$right';
  }
}
