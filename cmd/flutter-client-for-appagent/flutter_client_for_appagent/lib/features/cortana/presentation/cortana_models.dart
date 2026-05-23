part of 'cortana_page.dart';

class FlutterClientLogEntry {
  const FlutterClientLogEntry({required this.timestamp, required this.message});
  final DateTime timestamp;
  final String message;

  String get timeLabel {
    final hh = timestamp.hour.toString().padLeft(2, '0');
    final mm = timestamp.minute.toString().padLeft(2, '0');
    final ss = timestamp.second.toString().padLeft(2, '0');
    return '$hh:$mm:$ss';
  }
}

final List<FlutterClientLogEntry> flutterClientLogs = <FlutterClientLogEntry>[];
const int _maxFlutterClientLogs = 200;

void addFlutterClientLog(String message) {
  final text = sanitizeWellFormedUtf16(message).trim();
  if (text.isEmpty) return;
  final entry = FlutterClientLogEntry(timestamp: DateTime.now(), message: text);
  flutterClientLogs.insert(0, entry);
  if (flutterClientLogs.length > _maxFlutterClientLogs) {
    flutterClientLogs.removeRange(
      _maxFlutterClientLogs,
      flutterClientLogs.length,
    );
  }
}

enum CortanaDisplayMode { fullscreen, expanded, small, collapsed }

class CortanaModelViewTransform {
  const CortanaModelViewTransform({
    this.scale = 1.0,
    this.offsetX = 0.0,
    this.offsetY = 0.0,
  });

  final double scale;
  final double offsetX;
  final double offsetY;

  static const CortanaModelViewTransform defaults = CortanaModelViewTransform();

  CortanaModelViewTransform normalized() {
    return CortanaModelViewTransform(
      scale: scale.clamp(0.8, 1.35).toDouble(),
      offsetX: offsetX.clamp(-0.35, 0.35).toDouble(),
      offsetY: offsetY.clamp(-0.28, 0.28).toDouble(),
    );
  }

  Map<String, dynamic> toJson() {
    final value = normalized();
    return <String, dynamic>{
      'scale': value.scale,
      'offset_x': value.offsetX,
      'offset_y': value.offsetY,
    };
  }

  static CortanaModelViewTransform? fromJson(Map<String, dynamic> json) {
    double? readDouble(String key) {
      final raw = json[key];
      if (raw is num) {
        return raw.toDouble();
      }
      return double.tryParse((raw ?? '').toString());
    }

    final scale = readDouble('scale');
    final offsetX = readDouble('offset_x') ?? readDouble('offsetX');
    final offsetY = readDouble('offset_y') ?? readDouble('offsetY');
    if (scale == null || offsetX == null || offsetY == null) {
      return null;
    }
    return CortanaModelViewTransform(
      scale: scale,
      offsetX: offsetX,
      offsetY: offsetY,
    ).normalized();
  }

  @override
  bool operator ==(Object other) {
    return other is CortanaModelViewTransform &&
        other.scale == scale &&
        other.offsetX == offsetX &&
        other.offsetY == offsetY;
  }

  @override
  int get hashCode => Object.hash(scale, offsetX, offsetY);
}

class CortanaSettings {
  const CortanaSettings({
    this.enabled = true,
    this.allowFullAccess = true,
    this.autoPlay = true,
    this.proactiveMode = 'high',
    this.highFreqStartHour = 9,
    this.highFreqStartMinute = 0,
    this.highFreqEndHour = 22,
    this.highFreqEndMinute = 0,
    this.personaName = 'Cortana',
    this.ownerTitle = '',
    this.personaDescription = '',
    this.voiceWakeEnabled = false,
    this.wakePhrase = '嗨 Cortana',
  });

  final bool enabled;
  final bool allowFullAccess;
  final bool autoPlay;
  final String proactiveMode;
  final int highFreqStartHour;
  final int highFreqStartMinute;
  final int highFreqEndHour;
  final int highFreqEndMinute;
  final String personaName;
  final String ownerTitle;
  final String personaDescription;
  final bool voiceWakeEnabled;
  final String wakePhrase;

  CortanaSettings copyWith({
    bool? enabled,
    bool? allowFullAccess,
    bool? autoPlay,
    String? proactiveMode,
    int? highFreqStartHour,
    int? highFreqStartMinute,
    int? highFreqEndHour,
    int? highFreqEndMinute,
    String? personaName,
    String? ownerTitle,
    String? personaDescription,
    bool? voiceWakeEnabled,
    String? wakePhrase,
  }) {
    return CortanaSettings(
      enabled: enabled ?? this.enabled,
      allowFullAccess: allowFullAccess ?? this.allowFullAccess,
      autoPlay: autoPlay ?? this.autoPlay,
      proactiveMode: proactiveMode ?? this.proactiveMode,
      highFreqStartHour: highFreqStartHour ?? this.highFreqStartHour,
      highFreqStartMinute: highFreqStartMinute ?? this.highFreqStartMinute,
      highFreqEndHour: highFreqEndHour ?? this.highFreqEndHour,
      highFreqEndMinute: highFreqEndMinute ?? this.highFreqEndMinute,
      personaName: personaName ?? this.personaName,
      ownerTitle: ownerTitle ?? this.ownerTitle,
      personaDescription: personaDescription ?? this.personaDescription,
      voiceWakeEnabled: voiceWakeEnabled ?? this.voiceWakeEnabled,
      wakePhrase: wakePhrase ?? this.wakePhrase,
    );
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'enabled': enabled,
      'allow_full_access': allowFullAccess,
      'auto_play': autoPlay,
      'proactive_mode': proactiveMode,
      'high_freq_start_hour': highFreqStartHour,
      'high_freq_start_minute': highFreqStartMinute,
      'high_freq_end_hour': highFreqEndHour,
      'high_freq_end_minute': highFreqEndMinute,
      'persona_name': personaName,
      'owner_title': ownerTitle,
      'persona_description': personaDescription,
      'voice_wake_enabled': voiceWakeEnabled,
      'wake_phrase': wakePhrase,
    };
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is CortanaSettings &&
        other.enabled == enabled &&
        other.allowFullAccess == allowFullAccess &&
        other.autoPlay == autoPlay &&
        other.proactiveMode == proactiveMode &&
        other.highFreqStartHour == highFreqStartHour &&
        other.highFreqStartMinute == highFreqStartMinute &&
        other.highFreqEndHour == highFreqEndHour &&
        other.highFreqEndMinute == highFreqEndMinute &&
        other.personaName == personaName &&
        other.ownerTitle == ownerTitle &&
        other.personaDescription == personaDescription &&
        other.voiceWakeEnabled == voiceWakeEnabled &&
        other.wakePhrase == wakePhrase;
  }

  @override
  int get hashCode => Object.hash(
    enabled,
    allowFullAccess,
    autoPlay,
    proactiveMode,
    highFreqStartHour,
    highFreqStartMinute,
    highFreqEndHour,
    highFreqEndMinute,
    personaName,
    ownerTitle,
    personaDescription,
    voiceWakeEnabled,
    wakePhrase,
  );
}

class CortanaReplyPayload {
  const CortanaReplyPayload({
    required this.text,
    this.audioPath = '',
    this.audioBytes,
    this.audioFormat = '',
    this.actionPlan,
    this.suggestedReplies = const <CortanaSuggestedReply>[],
    this.requestId = '',
  });

  final String text;
  final String audioPath;
  final Uint8List? audioBytes;
  final String audioFormat;
  final Map<String, dynamic>? actionPlan;
  final List<CortanaSuggestedReply> suggestedReplies;
  final String requestId;

  bool get hasAudio => audioPath.trim().isNotEmpty || audioBytes != null;
}

class CortanaSuggestedReply {
  const CortanaSuggestedReply({
    required this.label,
    required this.message,
    this.kind = '',
  });

  final String label;
  final String message;
  final String kind;

  bool get isCustom => kind.trim().toLowerCase() == 'custom';

  bool get isNegativeAcknowledgement {
    final normalizedKind = kind.trim().toLowerCase();
    if (normalizedKind == 'negative' ||
        normalizedKind == 'decline' ||
        normalizedKind == 'cancel' ||
        normalizedKind == 'dismiss') {
      return true;
    }
    final normalizedLabel = label.trim().toLowerCase();
    final normalizedMessage = message.trim().toLowerCase();
    const negativeValues = <String>{
      '不',
      '否',
      '不用',
      '不要',
      '不想',
      '不了',
      '不需要',
      '先不用',
      '不用了',
      '算了',
      '取消',
      'no',
      'nope',
      'cancel',
      'decline',
      'dismiss',
    };
    bool isNegativeText(String value) {
      if (negativeValues.contains(value)) {
        return true;
      }
      return value.startsWith('不想') ||
          value.startsWith('不用') ||
          value.startsWith('不要') ||
          value.startsWith('先不用');
    }

    return isNegativeText(normalizedLabel) || isNegativeText(normalizedMessage);
  }

  factory CortanaSuggestedReply.fromMap(Map<String, dynamic> raw) {
    final label = (raw['label'] ?? raw['title'] ?? raw['text'] ?? '')
        .toString()
        .trim();
    final message = (raw['message'] ?? raw['value'] ?? raw['content'] ?? label)
        .toString()
        .trim();
    return CortanaSuggestedReply(
      label: label,
      message: message,
      kind: (raw['kind'] ?? raw['type'] ?? '').toString().trim(),
    );
  }
}

class CortanaReplayItem {
  const CortanaReplayItem({
    required this.id,
    required this.text,
    required this.audioPath,
    this.audioBytes,
    required this.audioFormat,
    required this.createdAt,
    this.actionPlan,
    this.sourceLabel = '',
    this.fileId = '',
    this.storageProvider = '',
    this.objectKey = '',
  });

  final String id;
  final String text;
  final String audioPath;
  final Uint8List? audioBytes;
  final String audioFormat;
  final DateTime createdAt;
  final Map<String, dynamic>? actionPlan;
  final String sourceLabel;
  final String fileId;
  final String storageProvider;
  final String objectKey;
}

class CortanaLogSource {
  const CortanaLogSource({
    required this.name,
    required this.path,
    required this.description,
  });

  final String name;
  final String path;
  final String description;
}

class CortanaLogFile {
  const CortanaLogFile({
    required this.name,
    required this.path,
    required this.size,
    required this.modifiedAt,
    required this.modifiedText,
  });

  final String name;
  final String path;
  final int size;
  final DateTime? modifiedAt;
  final String modifiedText;
}

class _CortanaVoiceHistoryItem {
  const _CortanaVoiceHistoryItem({
    required this.id,
    required this.text,
    required this.audioPath,
    this.audioBytes,
    required this.audioFormat,
    required this.createdAt,
    this.actionPlan,
  });

  final String id;
  final String text;
  final String audioPath;
  final Uint8List? audioBytes;
  final String audioFormat;
  final DateTime createdAt;
  final Map<String, dynamic>? actionPlan;
}

class _QueuedBroadcast {
  const _QueuedBroadcast(this.payload, this.onFinished);

  final CortanaReplyPayload payload;
  final VoidCallback? onFinished;
}

class _CustomCortanaWebRoot {
  const _CustomCortanaWebRoot({required this.root, required this.modelUrl});

  final Directory root;
  final String modelUrl;
}

String cortanaFileSystemEntityName(FileSystemEntity entity) {
  final normalized = entity.path.replaceAll('\\', '/');
  final trimmed = normalized.endsWith('/')
      ? normalized.substring(0, normalized.length - 1)
      : normalized;
  final index = trimmed.lastIndexOf('/');
  return index < 0 ? trimmed : trimmed.substring(index + 1);
}

Future<void> copyCortanaDirectoryForWebRuntime(
  Directory source,
  Directory destination,
) async {
  await for (final entity in source.list(
    recursive: false,
    followLinks: false,
  )) {
    final name = cortanaFileSystemEntityName(entity);
    if (name.isEmpty) {
      continue;
    }
    final outPath = '${destination.path}${Platform.pathSeparator}$name';
    if (entity is Directory) {
      final outDir = Directory(outPath);
      await outDir.create(recursive: true);
      await copyCortanaDirectoryForWebRuntime(entity, outDir);
    } else if (entity is File) {
      await File(outPath).writeAsBytes(await entity.readAsBytes());
    }
  }
}

Future<bool> addMissingCortanaModelReferencesForWebRuntime(
  File modelFile,
) async {
  if (!await modelFile.exists()) {
    return false;
  }

  final raw = await modelFile.readAsString(encoding: utf8);
  final decoded = jsonDecode(raw);
  if (decoded is! Map) {
    return false;
  }

  final modelJson = Map<String, dynamic>.from(decoded);
  final fileReferencesRaw = modelJson['FileReferences'];
  final fileReferences = fileReferencesRaw is Map
      ? Map<String, dynamic>.from(fileReferencesRaw)
      : <String, dynamic>{};
  var changed = false;

  final expressionFiles = await _listLive2dReferenceFiles(
    modelFile.parent,
    '.exp3.json',
  );
  if (expressionFiles.isNotEmpty) {
    final existingExpressions = _normalizeLive2dExpressionRefs(
      fileReferences['Expressions'],
    );
    final seen = existingExpressions
        .map((item) => (item['File'] ?? '').toString())
        .where((path) => path.isNotEmpty)
        .toSet();
    for (final path in expressionFiles) {
      if (!seen.add(path)) {
        continue;
      }
      existingExpressions.add(<String, String>{
        'Name': _live2dReferenceName(path, '.exp3.json'),
        'File': path,
      });
      changed = true;
    }
    if (existingExpressions.isNotEmpty) {
      fileReferences['Expressions'] = existingExpressions;
    }
  }

  final motionFiles = await _listLive2dReferenceFiles(
    modelFile.parent,
    '.motion3.json',
  );
  if (motionFiles.isNotEmpty) {
    final existingMotions = _normalizeLive2dMotionRefs(
      fileReferences['Motions'],
    );
    final seen = <String>{};
    for (final items in existingMotions.values) {
      for (final item in items) {
        final path = (item['File'] ?? '').toString();
        if (path.isNotEmpty) {
          seen.add(path);
        }
      }
    }
    for (final path in motionFiles) {
      if (!seen.add(path)) {
        continue;
      }
      final group = _live2dMotionGroupForPath(path);
      existingMotions.putIfAbsent(group, () => <Map<String, String>>[]).add(
        <String, String>{'File': path},
      );
      changed = true;
    }
    if (existingMotions.isNotEmpty) {
      fileReferences['Motions'] = existingMotions;
    }
  }

  if (!changed) {
    return false;
  }

  modelJson['FileReferences'] = fileReferences;
  const encoder = JsonEncoder.withIndent('  ');
  await modelFile.writeAsString(
    '${encoder.convert(modelJson)}\n',
    encoding: utf8,
  );
  return true;
}

Future<int> slowLive2dMotionFilesForWebRuntime(
  Directory baseDir, {
  double playbackSpeed = 0.55,
}) async {
  final normalizedSpeed = playbackSpeed.clamp(0.25, 1.0).toDouble();
  final timeScale = 1.0 / normalizedSpeed;
  var changedCount = 0;
  final files = await _listLive2dReferenceFiles(baseDir, '.motion3.json');
  for (final path in files) {
    final file = _resolveCortanaRelativeFile(baseDir, path);
    if (await _scaleLive2dMotionFile(file, timeScale)) {
      changedCount++;
    }
  }
  return changedCount;
}

Future<bool> _scaleLive2dMotionFile(File file, double timeScale) async {
  if (!await file.exists()) {
    return false;
  }
  final raw = await file.readAsString(encoding: utf8);
  final decoded = jsonDecode(raw);
  if (decoded is! Map) {
    return false;
  }
  final motionJson = Map<String, dynamic>.from(decoded);
  final metaRaw = motionJson['Meta'];
  final meta = metaRaw is Map ? Map<String, dynamic>.from(metaRaw) : null;
  final existingSpeed = meta == null
      ? null
      : double.tryParse('${meta['CortanaPlaybackSpeed'] ?? ''}');
  if (existingSpeed != null) {
    return false;
  }

  var changed = false;
  if (meta != null) {
    changed = _scaleNumberField(meta, 'Duration', timeScale) || changed;
    meta['CortanaPlaybackSpeed'] = (1.0 / timeScale).toStringAsFixed(2);
    motionJson['Meta'] = meta;
    changed = true;
  }

  final curves = motionJson['Curves'];
  if (curves is List) {
    for (var i = 0; i < curves.length; i++) {
      final curve = curves[i];
      if (curve is! Map) {
        continue;
      }
      final curveMap = Map<String, dynamic>.from(curve);
      if (_scaleMotionSegments(curveMap['Segments'], timeScale)) {
        curves[i] = curveMap;
        changed = true;
      }
    }
  }

  final events = motionJson['Events'];
  if (events is List) {
    for (var i = 0; i < events.length; i++) {
      final event = events[i];
      if (event is! Map) {
        continue;
      }
      final eventMap = Map<String, dynamic>.from(event);
      if (_scaleNumberField(eventMap, 'Time', timeScale)) {
        events[i] = eventMap;
        changed = true;
      }
    }
  }

  if (!changed) {
    return false;
  }
  const encoder = JsonEncoder.withIndent('  ');
  await file.writeAsString('${encoder.convert(motionJson)}\n', encoding: utf8);
  return true;
}

bool _scaleMotionSegments(Object? value, double timeScale) {
  if (value is! List || value.length < 2) {
    return false;
  }
  var changed = false;
  changed = _scaleNumberAt(value, 0, timeScale) || changed;
  var index = 2;
  while (index < value.length) {
    final segmentType = int.tryParse('${value[index]}');
    if (segmentType == null) {
      break;
    }
    if (segmentType == 0 || segmentType == 2 || segmentType == 3) {
      changed = _scaleNumberAt(value, index + 1, timeScale) || changed;
      index += 3;
      continue;
    }
    if (segmentType == 1) {
      changed = _scaleNumberAt(value, index + 1, timeScale) || changed;
      changed = _scaleNumberAt(value, index + 3, timeScale) || changed;
      changed = _scaleNumberAt(value, index + 5, timeScale) || changed;
      index += 7;
      continue;
    }
    break;
  }
  return changed;
}

bool _scaleNumberField(Map<String, dynamic> map, String key, double timeScale) {
  final value = map[key];
  if (value is! num) {
    return false;
  }
  map[key] = _scaledMotionTime(value, timeScale);
  return true;
}

bool _scaleNumberAt(List<dynamic> values, int index, double timeScale) {
  if (index < 0 || index >= values.length || values[index] is! num) {
    return false;
  }
  values[index] = _scaledMotionTime(values[index] as num, timeScale);
  return true;
}

double _scaledMotionTime(num value, double timeScale) {
  return double.parse((value * timeScale).toStringAsFixed(4));
}

List<Map<String, String>> _normalizeLive2dExpressionRefs(Object? value) {
  if (value is! List) {
    return <Map<String, String>>[];
  }
  return value.whereType<Map>().map((item) {
    return item.map<String, String>(
      (key, itemValue) => MapEntry('$key', '$itemValue'),
    );
  }).toList();
}

Map<String, List<Map<String, String>>> _normalizeLive2dMotionRefs(
  Object? value,
) {
  if (value is! Map) {
    return <String, List<Map<String, String>>>{};
  }
  return value.map<String, List<Map<String, String>>>((key, rawItems) {
    final items = rawItems is List
        ? rawItems.whereType<Map>().map((item) {
            return item.map<String, String>(
              (itemKey, itemValue) => MapEntry('$itemKey', '$itemValue'),
            );
          }).toList()
        : <Map<String, String>>[];
    return MapEntry('$key', items);
  });
}

Future<List<String>> _listLive2dReferenceFiles(
  Directory directory,
  String suffix,
) async {
  final paths = <String>[];
  await for (final entity in directory.list(
    recursive: true,
    followLinks: false,
  )) {
    if (entity is! File) {
      continue;
    }
    if (entity.path.toLowerCase().endsWith(suffix)) {
      paths.add(_cortanaRelativePath(directory, entity));
    }
  }
  paths.sort();
  return paths;
}

String _live2dReferenceName(String path, String suffix) {
  final lower = path.toLowerCase();
  if (!lower.endsWith(suffix)) {
    return path;
  }
  return path.substring(0, path.length - suffix.length);
}

String _live2dMotionGroupForPath(String path) {
  final lower = path.toLowerCase();
  if (lower.contains('tap') || lower.contains('touch')) {
    return 'Tap';
  }
  if (lower.contains('wave') || lower.contains('greet')) {
    return 'IdleWave';
  }
  return 'Idle';
}

String _cortanaRelativePath(Directory baseDir, File file) {
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
  return Uri(pathSegments: fileSegments.skip(index)).toString();
}

File _resolveCortanaRelativeFile(Directory baseDir, String relativePath) {
  final segments = Uri.parse(relativePath).pathSegments;
  return File(<String>[baseDir.path, ...segments].join(Platform.pathSeparator));
}
