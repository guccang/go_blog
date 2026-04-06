import 'dart:async';
import 'dart:convert'
    show base64Decode, base64Encode, jsonDecode, jsonEncode, utf8;
import 'dart:io';
import 'dart:math' as math;

import 'package:archive/archive.dart';
import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:speech_to_text/speech_to_text.dart' as stt;

import 'speech_transcript_formatter.dart';
import 'version.g.dart';
import 'vosk_model_locator.dart';

void main() {
  runApp(const AppAgentClientApp());
}

class UiThemePreset {
  const UiThemePreset({
    required this.id,
    required this.label,
    required this.accent,
    required this.brightness,
  });

  final String id;
  final String label;
  final Color accent;
  final Brightness brightness;
}

const List<UiThemePreset> kUiThemePresets = <UiThemePreset>[
  UiThemePreset(
    id: 'forest',
    label: '森林绿',
    accent: Color(0xFF2A8C6A),
    brightness: Brightness.dark,
  ),
  UiThemePreset(
    id: 'ocean',
    label: '海蓝',
    accent: Color(0xFF2F6FEB),
    brightness: Brightness.dark,
  ),
  UiThemePreset(
    id: 'sunset',
    label: '落日橙',
    accent: Color(0xFFCF7A37),
    brightness: Brightness.dark,
  ),
  UiThemePreset(
    id: 'ruby',
    label: '酒红',
    accent: Color(0xFFB44C6B),
    brightness: Brightness.dark,
  ),
  UiThemePreset(
    id: 'midnight',
    label: '午夜黑',
    accent: Color(0xFFF2F2F2),
    brightness: Brightness.dark,
  ),
  UiThemePreset(
    id: 'daylight',
    label: '日光白',
    accent: Color(0xFF181818),
    brightness: Brightness.light,
  ),
];

Color _blendWithAccent(Color base, Color accent, double alpha) {
  return Color.alphaBlend(accent.withValues(alpha: alpha), base);
}

Color _foregroundForColor(Color background) {
  return background.computeLuminance() > 0.44
      ? const Color(0xFF111111)
      : Colors.white;
}

Color _toneFromAccent(
  HSLColor accentHsl, {
  required double lightness,
  double saturationFactor = 0.35,
}) {
  final saturation = accentHsl.saturation < 0.06
      ? 0.0
      : (accentHsl.saturation * saturationFactor).clamp(0.08, 0.42).toDouble();
  return accentHsl
      .withSaturation(saturation)
      .withLightness(lightness.clamp(0.0, 1.0))
      .toColor();
}

UiThemePreset uiThemePresetFromId(String? id) {
  for (final preset in kUiThemePresets) {
    if (preset.id == id) {
      return preset;
    }
  }
  return kUiThemePresets.first;
}

@immutable
class AppPalette extends ThemeExtension<AppPalette> {
  const AppPalette({
    required this.backgroundTop,
    required this.backgroundBottom,
    required this.surface,
    required this.surfaceRaised,
    required this.surfaceMuted,
    required this.surfaceSoft,
    required this.border,
    required this.borderStrong,
    required this.textPrimary,
    required this.textSecondary,
    required this.textMuted,
    required this.accent,
    required this.accentSoft,
    required this.accentStrong,
    required this.success,
    required this.warning,
    required this.error,
    required this.messageIncoming,
    required this.messageSystem,
    required this.messageOutgoing,
  });

  factory AppPalette.fromPreset(UiThemePreset preset) {
    final accent = preset.accent;
    final accentHsl = HSLColor.fromColor(accent);
    final isDark = preset.brightness == Brightness.dark;
    final backgroundTop = isDark
        ? _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.12, saturationFactor: 0.34),
            accent,
            0.08,
          )
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.975, saturationFactor: 0.14),
            accent,
            0.03,
          );
    final backgroundBottom = isDark
        ? _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.055, saturationFactor: 0.28),
            accent,
            0.04,
          )
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.93, saturationFactor: 0.12),
            accent,
            0.05,
          );
    final surface = isDark
        ? _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.15, saturationFactor: 0.34),
            accent,
            0.10,
          )
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.99, saturationFactor: 0.06),
            accent,
            0.015,
          );
    final surfaceRaised = isDark
        ? _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.19, saturationFactor: 0.38),
            accent,
            0.14,
          )
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.97, saturationFactor: 0.08),
            accent,
            0.03,
          );
    final surfaceMuted = isDark
        ? _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.10, saturationFactor: 0.30),
            accent,
            0.08,
          )
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.945, saturationFactor: 0.10),
            accent,
            0.04,
          );
    final surfaceSoft = isDark
        ? _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.24, saturationFactor: 0.42),
            accent,
            0.18,
          )
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.90, saturationFactor: 0.14),
            accent,
            0.08,
          );
    final border = isDark
        ? _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.31, saturationFactor: 0.34),
            accent,
            0.14,
          )
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.82, saturationFactor: 0.12),
            accent,
            0.08,
          );
    final borderStrong = isDark
        ? _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.40, saturationFactor: 0.38),
            accent,
            0.18,
          )
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.68, saturationFactor: 0.14),
            accent,
            0.12,
          );
    final textPrimary = isDark
        ? _toneFromAccent(
            accentHsl,
            lightness: 0.965,
            saturationFactor: 0.06,
          )
        : _toneFromAccent(
            accentHsl,
            lightness: 0.10,
            saturationFactor: 0.10,
          );
    final textSecondary = isDark
        ? _toneFromAccent(
            accentHsl,
            lightness: 0.78,
            saturationFactor: 0.12,
          )
        : _toneFromAccent(
            accentHsl,
            lightness: 0.30,
            saturationFactor: 0.10,
          );
    final textMuted = isDark
        ? _toneFromAccent(
            accentHsl,
            lightness: 0.62,
            saturationFactor: 0.16,
          )
        : _toneFromAccent(
            accentHsl,
            lightness: 0.48,
            saturationFactor: 0.10,
          );
    final accentSoft = isDark
        ? _blendWithAccent(surfaceRaised, accent, 0.22)
        : _blendWithAccent(surfaceSoft, accent, 0.12);
    final accentStrong = isDark
        ? _blendWithAccent(surfaceSoft, accent, 0.68)
        : _blendWithAccent(
            _toneFromAccent(accentHsl, lightness: 0.18, saturationFactor: 0.18),
            accent,
            0.44,
          );
    final messageIncoming = isDark
        ? _blendWithAccent(surfaceRaised, accent, 0.08)
        : surfaceRaised;
    final messageSystem = isDark
        ? _blendWithAccent(surfaceSoft, accent, 0.12)
        : _blendWithAccent(surfaceSoft, accent, 0.08);
    final messageOutgoing = isDark
        ? _blendWithAccent(surfaceMuted, accent, 0.74)
        : accentStrong;
    return AppPalette(
      backgroundTop: backgroundTop,
      backgroundBottom: backgroundBottom,
      surface: surface,
      surfaceRaised: surfaceRaised,
      surfaceMuted: surfaceMuted,
      surfaceSoft: surfaceSoft,
      border: border,
      borderStrong: borderStrong,
      textPrimary: textPrimary,
      textSecondary: textSecondary,
      textMuted: textMuted,
      accent: accent,
      accentSoft: accentSoft,
      accentStrong: accentStrong,
      success: const Color(0xFF3EB489),
      warning: const Color(0xFFF0A04B),
      error: const Color(0xFFFF6B6B),
      messageIncoming: messageIncoming,
      messageSystem: messageSystem,
      messageOutgoing: messageOutgoing,
    );
  }

  final Color backgroundTop;
  final Color backgroundBottom;
  final Color surface;
  final Color surfaceRaised;
  final Color surfaceMuted;
  final Color surfaceSoft;
  final Color border;
  final Color borderStrong;
  final Color textPrimary;
  final Color textSecondary;
  final Color textMuted;
  final Color accent;
  final Color accentSoft;
  final Color accentStrong;
  final Color success;
  final Color warning;
  final Color error;
  final Color messageIncoming;
  final Color messageSystem;
  final Color messageOutgoing;

  @override
  AppPalette copyWith({
    Color? backgroundTop,
    Color? backgroundBottom,
    Color? surface,
    Color? surfaceRaised,
    Color? surfaceMuted,
    Color? surfaceSoft,
    Color? border,
    Color? borderStrong,
    Color? textPrimary,
    Color? textSecondary,
    Color? textMuted,
    Color? accent,
    Color? accentSoft,
    Color? accentStrong,
    Color? success,
    Color? warning,
    Color? error,
    Color? messageIncoming,
    Color? messageSystem,
    Color? messageOutgoing,
  }) {
    return AppPalette(
      backgroundTop: backgroundTop ?? this.backgroundTop,
      backgroundBottom: backgroundBottom ?? this.backgroundBottom,
      surface: surface ?? this.surface,
      surfaceRaised: surfaceRaised ?? this.surfaceRaised,
      surfaceMuted: surfaceMuted ?? this.surfaceMuted,
      surfaceSoft: surfaceSoft ?? this.surfaceSoft,
      border: border ?? this.border,
      borderStrong: borderStrong ?? this.borderStrong,
      textPrimary: textPrimary ?? this.textPrimary,
      textSecondary: textSecondary ?? this.textSecondary,
      textMuted: textMuted ?? this.textMuted,
      accent: accent ?? this.accent,
      accentSoft: accentSoft ?? this.accentSoft,
      accentStrong: accentStrong ?? this.accentStrong,
      success: success ?? this.success,
      warning: warning ?? this.warning,
      error: error ?? this.error,
      messageIncoming: messageIncoming ?? this.messageIncoming,
      messageSystem: messageSystem ?? this.messageSystem,
      messageOutgoing: messageOutgoing ?? this.messageOutgoing,
    );
  }

  @override
  AppPalette lerp(covariant ThemeExtension<AppPalette>? other, double t) {
    if (other is! AppPalette) {
      return this;
    }
    return AppPalette(
      backgroundTop: Color.lerp(backgroundTop, other.backgroundTop, t)!,
      backgroundBottom: Color.lerp(backgroundBottom, other.backgroundBottom, t)!,
      surface: Color.lerp(surface, other.surface, t)!,
      surfaceRaised: Color.lerp(surfaceRaised, other.surfaceRaised, t)!,
      surfaceMuted: Color.lerp(surfaceMuted, other.surfaceMuted, t)!,
      surfaceSoft: Color.lerp(surfaceSoft, other.surfaceSoft, t)!,
      border: Color.lerp(border, other.border, t)!,
      borderStrong: Color.lerp(borderStrong, other.borderStrong, t)!,
      textPrimary: Color.lerp(textPrimary, other.textPrimary, t)!,
      textSecondary: Color.lerp(textSecondary, other.textSecondary, t)!,
      textMuted: Color.lerp(textMuted, other.textMuted, t)!,
      accent: Color.lerp(accent, other.accent, t)!,
      accentSoft: Color.lerp(accentSoft, other.accentSoft, t)!,
      accentStrong: Color.lerp(accentStrong, other.accentStrong, t)!,
      success: Color.lerp(success, other.success, t)!,
      warning: Color.lerp(warning, other.warning, t)!,
      error: Color.lerp(error, other.error, t)!,
      messageIncoming: Color.lerp(messageIncoming, other.messageIncoming, t)!,
      messageSystem: Color.lerp(messageSystem, other.messageSystem, t)!,
      messageOutgoing: Color.lerp(messageOutgoing, other.messageOutgoing, t)!,
    );
  }
}

extension AppPaletteContext on BuildContext {
  AppPalette get appPalette => Theme.of(this).extension<AppPalette>()!;
}

ThemeData _buildAppTheme(UiThemePreset preset) {
  final palette = AppPalette.fromPreset(preset);
  final colorScheme = (preset.brightness == Brightness.dark
          ? const ColorScheme.dark()
          : const ColorScheme.light())
      .copyWith(
        brightness: preset.brightness,
        primary: palette.accent,
        secondary: palette.accent,
        primaryContainer: palette.accentStrong,
        secondaryContainer: palette.accentSoft,
        surface: palette.surface,
        error: palette.error,
        onPrimary: _foregroundForColor(palette.accent),
        onSecondary: _foregroundForColor(palette.accent),
        onPrimaryContainer: _foregroundForColor(palette.accentStrong),
        onSecondaryContainer: _foregroundForColor(palette.accentSoft),
        onSurface: palette.textPrimary,
        onError: Colors.white,
      );
  return ThemeData(
    useMaterial3: true,
    brightness: preset.brightness,
    colorScheme: colorScheme,
    scaffoldBackgroundColor: palette.backgroundBottom,
    canvasColor: palette.surface,
    splashColor: palette.accent.withValues(alpha: 0.14),
    highlightColor: palette.accent.withValues(alpha: 0.08),
    appBarTheme: AppBarTheme(
      backgroundColor: Colors.transparent,
      foregroundColor: palette.textPrimary,
      elevation: 0,
      centerTitle: false,
      systemOverlayStyle: preset.brightness == Brightness.dark
          ? SystemUiOverlayStyle.light
          : SystemUiOverlayStyle.dark,
      titleTextStyle: TextStyle(
        color: palette.textPrimary,
        fontSize: 20,
        fontWeight: FontWeight.w700,
      ),
    ),
    cardTheme: CardThemeData(
      color: palette.surfaceRaised,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
      surfaceTintColor: Colors.transparent,
    ),
    inputDecorationTheme: InputDecorationTheme(
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(18),
        borderSide: BorderSide(color: palette.borderStrong),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(18),
        borderSide: BorderSide(color: palette.borderStrong),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(18),
        borderSide: BorderSide(color: palette.accent, width: 1.4),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(18),
        borderSide: BorderSide(color: palette.error),
      ),
      focusedErrorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(18),
        borderSide: BorderSide(color: palette.error, width: 1.4),
      ),
      filled: true,
      fillColor: palette.surfaceRaised,
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
      labelStyle: TextStyle(color: palette.textSecondary),
      hintStyle: TextStyle(color: palette.textMuted),
      prefixIconColor: palette.textSecondary,
      suffixIconColor: palette.textSecondary,
    ),
    chipTheme: ChipThemeData(
      backgroundColor: palette.surfaceRaised,
      selectedColor: palette.accentSoft,
      secondarySelectedColor: palette.accentSoft,
      disabledColor: palette.surfaceMuted,
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      labelStyle: TextStyle(
        color: palette.textSecondary,
        fontWeight: FontWeight.w600,
      ),
      secondaryLabelStyle: TextStyle(
        color: _foregroundForColor(palette.accentSoft),
        fontWeight: FontWeight.w700,
      ),
      side: BorderSide(color: palette.borderStrong),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      brightness: preset.brightness,
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: palette.accent,
        foregroundColor: _foregroundForColor(palette.accent),
        disabledBackgroundColor: palette.surfaceSoft,
        disabledForegroundColor: palette.textMuted,
        textStyle: const TextStyle(fontWeight: FontWeight.w700),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: palette.textPrimary,
        side: BorderSide(color: palette.borderStrong),
        textStyle: const TextStyle(fontWeight: FontWeight.w600),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        foregroundColor: palette.accent,
        textStyle: const TextStyle(fontWeight: FontWeight.w600),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      ),
    ),
    iconButtonTheme: IconButtonThemeData(
      style: IconButton.styleFrom(foregroundColor: palette.textSecondary),
    ),
    listTileTheme: ListTileThemeData(
      iconColor: palette.textSecondary,
      textColor: palette.textPrimary,
    ),
    progressIndicatorTheme: ProgressIndicatorThemeData(
      color: palette.accent,
      circularTrackColor: palette.surfaceMuted,
      linearTrackColor: palette.surfaceMuted,
    ),
    textSelectionTheme: TextSelectionThemeData(
      cursorColor: palette.accent,
      selectionColor: palette.accent.withValues(alpha: 0.22),
      selectionHandleColor: palette.accent,
    ),
    snackBarTheme: SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: palette.surfaceRaised,
      contentTextStyle: TextStyle(color: palette.textPrimary),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
    ),
    popupMenuTheme: PopupMenuThemeData(
      color: palette.surfaceMuted,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
    ),
    bottomSheetTheme: BottomSheetThemeData(
      backgroundColor: palette.surface,
      surfaceTintColor: Colors.transparent,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
    ),
    dividerColor: palette.border,
    extensions: <ThemeExtension<dynamic>>[palette],
  );
}

class AppAgentClientApp extends StatefulWidget {
  const AppAgentClientApp({super.key});

  @override
  State<AppAgentClientApp> createState() => _AppAgentClientAppState();
}

class _AppAgentClientAppState extends State<AppAgentClientApp> {
  static const String _uiThemePresetKey = 'ui_theme_preset';

  UiThemePreset _themePreset = kUiThemePresets.first;

  @override
  void initState() {
    super.initState();
    unawaited(_restoreThemePreset());
  }

  Future<void> _restoreThemePreset() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final savedId = prefs.getString(_uiThemePresetKey);
      if (!mounted) {
        return;
      }
      setState(() {
        _themePreset = uiThemePresetFromId(savedId);
      });
    } catch (_) {
      // Ignore theme restore failures and keep the default preset.
    }
  }

  void _handleThemePresetChanged(UiThemePreset preset) {
    if (_themePreset.id == preset.id) {
      return;
    }
    setState(() {
      _themePreset = preset;
    });
    unawaited(_persistThemePreset(preset));
  }

  Future<void> _persistThemePreset(UiThemePreset preset) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_uiThemePresetKey, preset.id);
    } catch (_) {
      // Ignore persistence failures for a purely visual preference.
    }
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'App Agent Client',
      debugShowCheckedModeBanner: false,
      theme: _buildAppTheme(_themePreset),
      home: ChatPage(
        themePreset: _themePreset,
        onThemePresetChanged: _handleThemePresetChanged,
      ),
    );
  }
}

class ChatMessage {
  ChatMessage({
    required this.content,
    required this.direction,
    required this.timestamp,
    this.status = 'sent',
    this.scopeKey = 'direct',
    this.authorId = '',
    this.groupId = '',
    this.messageType = 'text',
    this.meta,
  });

  final String content;
  final MessageDirection direction;
  final DateTime timestamp;
  final String status;
  final String scopeKey;
  final String authorId;
  final String groupId;
  final String messageType;
  final Map<String, dynamic>? meta;

  Map<String, dynamic> toJson() {
    final sanitizedMeta = meta == null
        ? null
        : Map<String, dynamic>.from(meta!);
    sanitizedMeta?.remove('audio_base64');
    return {
      'content': content,
      'direction': direction.name,
      'timestamp': timestamp.millisecondsSinceEpoch,
      'status': status,
      'scope_key': scopeKey,
      'author_id': authorId,
      'group_id': groupId,
      'message_type': messageType,
      'meta': sanitizedMeta,
    };
  }

  factory ChatMessage.fromJson(Map<String, dynamic> json) {
    final directionName = (json['direction'] ?? 'system').toString();
    final direction = MessageDirection.values.firstWhere(
      (value) => value.name == directionName,
      orElse: () => MessageDirection.system,
    );
    return ChatMessage(
      content: (json['content'] ?? '').toString(),
      direction: direction,
      timestamp: DateTime.fromMillisecondsSinceEpoch(
        json['timestamp'] is int
            ? json['timestamp'] as int
            : int.tryParse('${json['timestamp']}') ??
                  DateTime.now().millisecondsSinceEpoch,
      ),
      status: (json['status'] ?? 'sent').toString(),
      scopeKey: (json['scope_key'] ?? 'direct').toString(),
      authorId: (json['author_id'] ?? '').toString(),
      groupId: (json['group_id'] ?? '').toString(),
      messageType: (json['message_type'] ?? 'text').toString(),
      meta: json['meta'] is Map<String, dynamic>
          ? json['meta'] as Map<String, dynamic>
          : null,
    );
  }
}

bool isApkChatMessage(ChatMessage message) {
  if (message.messageType != 'file') {
    return false;
  }
  final fileName = (message.meta?['file_name'] ?? '')
      .toString()
      .trim()
      .toLowerCase();
  final fileFormat = (message.meta?['file_format'] ?? '')
      .toString()
      .trim()
      .toLowerCase();
  return fileName.endsWith('.apk') || fileFormat == 'apk';
}

/// Extract version string from APK filename.
/// Examples: "app-release-1.0.0.apk" -> "1.0.0", "myapp-2.3.4+5.apk" -> "2.3.4+5"
String? extractApkVersion(ChatMessage message) {
  final fileName = (message.meta?['file_name'] ?? '').toString().trim();
  final match = RegExp(
    r'[-_](\d+\.\d+\.\d+(?:\+\d+)?)[^.]*\.apk$',
    caseSensitive: false,
  ).firstMatch(fileName);
  return match?.group(1);
}

/// Extract version string from APK filename.
/// Examples: "app-release-1.0.0.apk" -> "1.0.0", "myapp-2.3.4+5.apk" -> "2.3.4+5"
String? extractApkVersionFromString(String fileName) {
  final match = RegExp(
    r'[-_](\d+\.\d+\.\d+(?:\+\d+)?)[^.]*\.apk$',
    caseSensitive: false,
  ).firstMatch(fileName);
  return match?.group(1);
}

/// Compare two version strings.
/// Returns 1 if versionA > versionB, 0 if equal, -1 if versionA < versionB.
int compareApkVersions(String? versionA, String? versionB) {
  if (versionA == null && versionB == null) return 0;
  if (versionA == null) return -1;
  if (versionB == null) return 1;

  // Parse version parts (e.g., "1.2.3" -> [1, 2, 3])
  List<int> parseParts(String v) {
    // Remove build metadata part after +
    final baseV = v.split('+')[0];
    final parts = baseV.split('.');
    final result = <int>[];
    for (final part in parts) {
      final num = int.tryParse(part);
      result.add(num ?? 0);
    }
    return result;
  }

  final partsA = parseParts(versionA);
  final partsB = parseParts(versionB);

  // Compare each part
  final maxLen = partsA.length > partsB.length ? partsA.length : partsB.length;
  for (int i = 0; i < maxLen; i++) {
    final valA = i < partsA.length ? partsA[i] : 0;
    final valB = i < partsB.length ? partsB[i] : 0;

    if (valA > valB) return 1;
    if (valA < valB) return -1;
  }

  return 0;
}

enum MessageDirection { outgoing, incoming, system }

enum _AttachmentMenuAction { galleryImage, cameraImage }

enum VoiceGestureAction { sendAudio, cancel, transcribe }

const double _voiceGestureVerticalThreshold = 48;
const double _voiceGestureHorizontalThreshold = 24;
const double _voiceGestureStrongHorizontalThreshold = 72;
const double _voiceGestureHorizontalGraceDy = 24;

VoiceGestureAction resolveVoiceGestureAction(Offset dragOffset) {
  final dx = dragOffset.dx;
  final dy = dragOffset.dy;
  final movedUp = dy <= -_voiceGestureVerticalThreshold;
  final movedLeft = dx <= -_voiceGestureHorizontalThreshold;
  final movedRight = dx >= _voiceGestureHorizontalThreshold;
  final strongLeftSwipe =
      dx <= -_voiceGestureStrongHorizontalThreshold &&
      dy <= _voiceGestureHorizontalGraceDy;
  final strongRightSwipe =
      dx >= _voiceGestureStrongHorizontalThreshold &&
      dy <= _voiceGestureHorizontalGraceDy;

  if ((movedUp && movedLeft) || strongLeftSwipe) {
    return VoiceGestureAction.cancel;
  }
  if ((movedUp && movedRight) || strongRightSwipe) {
    return VoiceGestureAction.transcribe;
  }
  return VoiceGestureAction.sendAudio;
}

class PushEnvelope {
  PushEnvelope({
    required this.messageId,
    required this.sequence,
    required this.userId,
    required this.content,
    required this.channel,
    required this.messageType,
    required this.timestamp,
    this.meta,
  });

  factory PushEnvelope.fromJson(Map<String, dynamic> json) {
    return PushEnvelope(
      messageId: (json['message_id'] ?? '').toString(),
      sequence: json['sequence'] is int
          ? json['sequence'] as int
          : int.tryParse('${json['sequence']}') ?? 0,
      userId: (json['user_id'] ?? '').toString(),
      content: (json['content'] ?? '').toString(),
      channel: (json['channel'] ?? '').toString(),
      messageType: (json['message_type'] ?? 'text').toString(),
      timestamp: json['timestamp'] is int
          ? json['timestamp'] as int
          : int.tryParse('${json['timestamp']}') ??
                DateTime.now().millisecondsSinceEpoch,
      meta: json['meta'] is Map<String, dynamic>
          ? json['meta'] as Map<String, dynamic>
          : null,
    );
  }

  final String messageId;
  final int sequence;
  final String userId;
  final String content;
  final String channel;
  final String messageType;
  final int timestamp;
  final Map<String, dynamic>? meta;
}

class RecordedAudio {
  const RecordedAudio({required this.path, required this.duration});

  final String path;
  final Duration duration;
}

class GroupInfo {
  const GroupInfo({
    required this.id,
    required this.members,
    required this.createdAt,
  });

  final String id;
  final List<String> members;
  final DateTime createdAt;

  factory GroupInfo.fromJson(Map<String, dynamic> json) {
    final members = (json['members'] as List<dynamic>? ?? const [])
        .map((item) => item.toString())
        .toList();
    return GroupInfo(
      id: (json['id'] ?? '').toString(),
      members: members,
      createdAt: DateTime.fromMillisecondsSinceEpoch(
        json['created_at'] is int
            ? json['created_at'] as int
            : int.tryParse('${json['created_at']}') ??
                  DateTime.now().millisecondsSinceEpoch,
      ),
    );
  }
}

class ClientConfig {
  const ClientConfig({
    required this.baseUrl,
    required this.receiveToken,
    required this.enableLocalVosk,
    required this.voskModelPath,
  });

  final String baseUrl;
  final String receiveToken;
  final bool enableLocalVosk;
  final String voskModelPath;

  factory ClientConfig.fromJson(Map<String, dynamic> json) {
    return ClientConfig(
      baseUrl: (json['base_url'] ?? '').toString().trim(),
      receiveToken: (json['receive_token'] ?? '').toString().trim(),
      enableLocalVosk: json['enable_local_vosk'] == true,
      voskModelPath: (json['vosk_model_path'] ?? '').toString().trim(),
    );
  }
}

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

class _SignedAttachmentDownload {
  const _SignedAttachmentDownload({
    required this.uri,
    required this.headersBuilder,
  });

  final Uri uri;
  final DownloadHeadersBuilder headersBuilder;
}

class AppAgentClient {
  static const Duration _httpTimeout = Duration(seconds: 8);
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

  Map<String, String> _obsAgentHeaders() {
    return <String, String>{
      if (receiveToken.trim().isNotEmpty)
        'X-App-Agent-Token': receiveToken.trim(),
    };
  }

  Future<Map<String, dynamic>> login() async {
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
      throw HttpException('login failed: ${resp.statusCode} ${resp.body}');
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
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
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
            if (sessionToken.trim().isNotEmpty)
              'X-App-Agent-Session': sessionToken.trim(),
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
      throw HttpException('send failed: ${resp.statusCode} ${resp.body}');
    }
  }

  Future<void> sendMessage(String content) => sendAppMessage(content);

  Future<List<GroupInfo>> listGroups() async {
    final uri = Uri.parse(
      '$baseUrl/api/app/groups?user_id=$userId&session_token=$sessionToken',
    );
    final resp = await http
        .get(
          uri,
          headers: {
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
            if (sessionToken.trim().isNotEmpty)
              'X-App-Agent-Session': sessionToken.trim(),
          },
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw HttpException(
        'list groups failed: ${resp.statusCode} ${resp.body}',
      );
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
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
            if (sessionToken.trim().isNotEmpty)
              'X-App-Agent-Session': sessionToken.trim(),
          },
          body: jsonEncode({
            'action': action,
            'user_id': userId,
            'group_id': groupId,
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw HttpException(
        'group $action failed: ${resp.statusCode} ${resp.body}',
      );
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final groups = (data['groups'] as List<dynamic>? ?? const [])
        .map((item) => GroupInfo.fromJson(item as Map<String, dynamic>))
        .toList();
    return groups;
  }

  Future<WebSocket> connectWebSocket() {
    final uri = _buildWsUri(baseUrl, userId, sessionToken);
    return WebSocket.connect(
      uri.toString(),
      headers: {
        if (receiveToken.trim().isNotEmpty)
          'X-App-Agent-Token': receiveToken.trim(),
        if (sessionToken.trim().isNotEmpty)
          'X-App-Agent-Session': sessionToken.trim(),
      },
    ).timeout(_wsConnectTimeout);
  }

  Future<List<int>> downloadAttachment(String fileId) async {
    final uri = _buildAttachmentUri(fileId);
    final resp = await http.get(uri, headers: _attachmentHeaders());
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
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
    final meta = attachmentMeta == null
        ? const <String, dynamic>{}
        : Map<String, dynamic>.from(attachmentMeta);

    if (_shouldTryObsDownload(meta)) {
      final downloadTicket = (meta['download_ticket'] ?? '').toString().trim();
      try {
        final signed = await _requestObsDownload(
          fileId,
          downloadTicket: downloadTicket,
        );
        await downloader.downloadToFile(
          signed.uri,
          destinationPath: destinationPath,
          headersBuilder: signed.headersBuilder,
          onProgress: onProgress,
        );
        return;
      } catch (_) {
        // Fall back to the legacy app-agent attachment endpoint.
      }
    }

    final uri = _buildAttachmentUri(fileId);
    await downloader.downloadToFile(
      uri,
      destinationPath: destinationPath,
      headersBuilder: ({int? rangeStart}) =>
          _attachmentHeaders(rangeStart: rangeStart),
      onProgress: onProgress,
    );
  }

  bool _shouldTryObsDownload(Map<String, dynamic> meta) {
    final storageProvider = (meta['storage_provider'] ?? '')
        .toString()
        .trim()
        .toLowerCase();
    final downloadTicket = (meta['download_ticket'] ?? '').toString().trim();
    final objectKey = (meta['object_key'] ?? '').toString().trim();
    return obsAgentBaseUrl.trim().isNotEmpty &&
        downloadTicket.isNotEmpty &&
        (storageProvider == 'obs' || objectKey.isNotEmpty);
  }

  Future<_SignedAttachmentDownload> _requestObsDownload(
    String fileId, {
    required String downloadTicket,
  }) async {
    final base = Uri.parse(obsAgentBaseUrl);
    final pathSegments = <String>[
      ...base.pathSegments.where((segment) => segment.isNotEmpty),
      'api',
      'obs',
      'download',
      fileId,
    ];
    final uri = base.replace(
      pathSegments: pathSegments,
      queryParameters: <String, String>{'ticket': downloadTicket},
    );
    final resp = await http
        .get(uri, headers: _obsAgentHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw HttpException(
        'obs download sign failed: ${resp.statusCode} ${resp.body}',
      );
    }

    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final url = (data['url'] ?? '').toString().trim();
    if (url.isEmpty) {
      throw const FormatException('missing signed download url');
    }
    final headerMap = <String, String>{};
    final rawHeaders = data['headers'];
    if (rawHeaders is Map) {
      for (final entry in rawHeaders.entries) {
        headerMap[entry.key.toString()] = (entry.value ?? '').toString();
      }
    }
    return _SignedAttachmentDownload(
      uri: Uri.parse(url),
      headersBuilder: ({int? rangeStart}) => <String, String>{
        ...headerMap,
        if (rangeStart != null && rangeStart > 0)
          HttpHeaders.rangeHeader: 'bytes=$rangeStart-',
      },
    );
  }

  static Uri _buildWsUri(String baseUrl, String userId, String sessionToken) {
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
        if (sessionToken.trim().isNotEmpty)
          'session_token': sessionToken.trim(),
      },
    );
  }
}

class ChatPage extends StatefulWidget {
  const ChatPage({
    super.key,
    required this.themePreset,
    required this.onThemePresetChanged,
  });

  final UiThemePreset themePreset;
  final ValueChanged<UiThemePreset> onThemePresetChanged;

  @override
  State<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends State<ChatPage> {
  static const String _baseUrlOverrideKey = 'client_config::base_url_override';
  static const List<Duration> _voskDownloadRetryDelays = <Duration>[
    Duration(seconds: 1),
    Duration(seconds: 2),
    Duration(seconds: 4),
  ];

  final _userIdController = TextEditingController(text: 'demo-user');
  final _passwordController = TextEditingController();
  final _baseUrlController = TextEditingController();
  final _groupIdController = TextEditingController();
  final _messageController = TextEditingController();
  final FocusNode _messageFocusNode = FocusNode();
  final _scrollController = ScrollController();
  final _controlsScrollController = ScrollController();
  final AudioRecorder _audioRecorder = AudioRecorder();
  final AudioPlayer _audioPlayer = AudioPlayer();
  final ImagePicker _imagePicker = ImagePicker();
  final stt.SpeechToText _speechToText = stt.SpeechToText();
  final VoskTranscriber _voskTranscriber = VoskTranscriber();
  final ApkInstaller _apkInstaller = ApkInstaller();
  final ZipExtractor _zipExtractor = ZipExtractor();
  final ResumableFileDownloader _fileDownloader = const ResumableFileDownloader(
    retryDelays: _voskDownloadRetryDelays,
  );

  final Map<String, List<ChatMessage>> _historyByScope =
      <String, List<ChatMessage>>{};
  final List<GroupInfo> _groups = <GroupInfo>[];
  final Set<String> _seenMessageIds = <String>{};
  final Set<String> _autoInstallTriggered = <String>{};

  WebSocket? _socket;
  StreamSubscription<dynamic>? _socketSub;
  Timer? _reconnectTimer;

  bool _connecting = false;
  bool _connected = false;
  bool _loggingIn = false;
  bool _recording = false;
  bool _speechReady = false;
  bool _useLocalVosk = false;
  bool _sending = false;
  bool _transcribingVoice = false;
  bool _voiceInputMode = false;
  String? _playingAudioKey;
  bool _autoReconnect = false;
  bool _configLoading = true;
  bool _controlsExpanded = false;
  bool _groupTabsExpanded = false;
  bool _passwordVisible = false;
  int _lastSequence = 0;
  String _status = 'Idle';
  String _sessionToken = '';
  String _obsAgentBaseUrl = '';
  String _currentGroupId = '';
  String _configError = '';
  Offset _recordDragOffset = Offset.zero;
  Offset? _recordDragStartGlobalPosition;
  String _speechDraft = '';
  DateTime? _recordStartedAt;
  ClientConfig? _clientConfig;
  String? _downloadStatusLabel;
  int _downloadStatusPercent = -1;
  bool _voskModelDownloading = false;
  double _voskModelDownloadProgress = 0.0;
  String? _voskModelDownloadError;

  @override
  void initState() {
    super.initState();
    _appendSystem('Loading client config...');
    unawaited(_loadClientConfig());
    unawaited(_restoreVoskDownloadProgress());
  }

  Future<void> _restoreVoskDownloadProgress() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await _migrateLegacyVoskPartialArchive(prefs);
      final partFile = await _getVoskArchivePartFile();
      if (await partFile.exists()) {
        final partialBytes = await partFile.length();
        if (partialBytes <= 0) {
          await partFile.delete();
          await prefs.remove(_voskDownloadProgressKey);
          await prefs.remove(_voskDownloadBytesKey);
          return;
        }
        final savedProgress = await _getVoskDownloadProgress();
        final savedBytes = prefs.getInt(_voskDownloadBytesKey) ?? partialBytes;
        if (!mounted) return;
        setState(() {
          _voskModelDownloadProgress = savedProgress > 0 && savedProgress < 1.0
              ? savedProgress
              : 0.0;
          _status = 'Vosk 模型下载未完成（已下载 ${_formatBytes(savedBytes)}），点击继续下载按钮可继续';
        });
        _appendSystem('检测到未完成的 Vosk 模型下载，可点击继续下载');
      } else {
        await prefs.remove(_voskDownloadProgressKey);
        await prefs.remove(_voskDownloadBytesKey);
      }
    } catch (_) {
      // Ignore errors during progress restoration
    }
  }

  @override
  void dispose() {
    _reconnectTimer?.cancel();
    unawaited(_socketSub?.cancel());
    unawaited(_socket?.close());
    _userIdController.dispose();
    _passwordController.dispose();
    _baseUrlController.dispose();
    _groupIdController.dispose();
    _messageController.dispose();
    _messageFocusNode.dispose();
    _scrollController.dispose();
    _controlsScrollController.dispose();
    unawaited(_audioPlayer.dispose());
    unawaited(_audioRecorder.dispose());
    super.dispose();
  }

  Future<void> _initVoice() async {
    final config = _clientConfig;
    final prefs = await SharedPreferences.getInstance();
    if (Platform.isAndroid && config != null && config.enableLocalVosk) {
      final modelPath = await _resolveAvailableVoskModelPath(
        preferredPath: config.voskModelPath,
      );
      if (modelPath != null) {
        final localModelPath = await _getLocalVoskModelPath();
        final savedModelPath = prefs.getString('vosk_model_path')?.trim() ?? '';
        final localModelPrefix = '$localModelPath${Platform.pathSeparator}';
        if ((modelPath == localModelPath ||
                modelPath.startsWith(localModelPrefix)) &&
            savedModelPath != modelPath) {
          await prefs.setString('vosk_model_path', modelPath);
        }
        try {
          final error = await _voskTranscriber.initialize(modelPath);
          if (!mounted) {
            return;
          }
          if (error == null) {
            setState(() {
              _speechReady = true;
              _useLocalVosk = true;
            });
            _appendSystem('Vosk local speech recognition is ready.');
            return;
          }
          await prefs.remove('vosk_model_path');
          if (!mounted) {
            return;
          }
          setState(() {
            _speechReady = false;
            _useLocalVosk = false;
          });
          _appendSystem(
            'Vosk model invalid, cleared model path. Please re-download: $error',
          );
        } catch (err) {
          await prefs.remove('vosk_model_path');
          if (!mounted) {
            return;
          }
          setState(() {
            _speechReady = false;
            _useLocalVosk = false;
          });
          _appendSystem(
            'Initialize Vosk failed, cleared model path. Please re-download: $err',
          );
        }
      } else if ((config.voskModelPath).trim().isNotEmpty) {
        await prefs.remove('vosk_model_path');
        _appendSystem(
          'Vosk model directory is incomplete, fallback to system speech recognition.',
        );
      }
    }

    try {
      final available = await _speechToText.initialize(
        onError: (error) {
          _appendSystem('Speech recognition error: $error');
        },
        onStatus: (status) {
          _appendSystem('Speech recognition status: $status');
        },
      );
      if (!mounted) {
        return;
      }
      if (!available) {
        _appendSystem('Speech recognition not available on this device.');
      }
      setState(() {
        _speechReady = available;
        _useLocalVosk = false;
      });
    } catch (err, stack) {
      if (!mounted) {
        return;
      }
      _appendSystem('Speech recognition init failed: $err');
      debugPrint('Speech init error: $err\n$stack');
      setState(() {
        _speechReady = false;
        _useLocalVosk = false;
      });
    }
  }

  static const String _voskModelUrl =
      'https://alphacephei.com/vosk/models/vosk-model-small-cn-0.22.zip';
  static const String _voskDownloadProgressKey = 'vosk_download_progress';
  static const String _voskDownloadBytesKey = 'vosk_download_bytes';

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
      if (Platform.isAndroid) {
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
      // Use saved model path if available (from downloaded model), otherwise use asset config
      final savedModelPath = prefs.getString('vosk_model_path')?.trim() ?? '';
      final config = ClientConfig(
        baseUrl: savedBaseUrl.isEmpty ? assetConfig.baseUrl : savedBaseUrl,
        receiveToken: assetConfig.receiveToken,
        enableLocalVosk: assetConfig.enableLocalVosk,
        voskModelPath: savedModelPath.isNotEmpty
            ? savedModelPath
            : assetConfig.voskModelPath,
      );
      if (config.baseUrl.isEmpty) {
        throw const FormatException('base_url is required');
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _clientConfig = config;
        _baseUrlController.text = config.baseUrl;
        _configLoading = false;
        _configError = '';
        _status = 'Config loaded';
      });
      _appendSystem('Client config loaded.');
      unawaited(_initVoice());
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
      );
      _status = 'URL updated';
    });
    _appendSystem('Server URL updated: $baseUrl');
  }

  AppAgentClient get _client => AppAgentClient(
    baseUrl: _clientConfig?.baseUrl ?? '',
    userId: _userIdController.text.trim(),
    password: _passwordController.text,
    receiveToken: _clientConfig?.receiveToken ?? '',
    sessionToken: _sessionToken,
    obsAgentBaseUrl: _obsAgentBaseUrl,
  );

  String get _currentScopeKey =>
      _currentGroupId.isEmpty ? 'direct' : _groupScopeKey(_currentGroupId);

  List<ChatMessage> get _messages =>
      _historyByScope[_currentScopeKey] ?? const <ChatMessage>[];

  String _resolvePreferredGroupId(
    List<GroupInfo> groups, {
    String? preferredGroupId,
  }) {
    if (groups.isEmpty) {
      return '';
    }
    final preferred = preferredGroupId?.trim() ?? '';
    if (preferred.isNotEmpty && groups.any((group) => group.id == preferred)) {
      return preferred;
    }
    if (groups.length == 1) {
      return groups.first.id;
    }
    return '';
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

  void _appendOutgoing(
    String text, {
    String messageType = 'text',
    Map<String, dynamic>? meta,
  }) {
    _appendMessage(
      ChatMessage(
        content: text,
        direction: MessageDirection.outgoing,
        timestamp: DateTime.now(),
        scopeKey: _currentScopeKey,
        authorId: _userIdController.text.trim(),
        groupId: _currentGroupId,
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
        unawaited(_persistHistory(message.scopeKey));
      }
      return;
    }
    setState(() {
      if (updateStatus != null) {
        _status = updateStatus;
      }
    });
    if (persist) {
      unawaited(_persistHistory(message.scopeKey));
    }
    if (message.scopeKey == _currentScopeKey) {
      _scrollToBottom();
    }
  }

  void _scrollToBottom({bool animated = true}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) {
        return;
      }
      final target = _scrollController.position.maxScrollExtent + 80;
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
    if (message.messageType == 'audio') {
      final meta = message.meta;
      final audioPath = (meta?['audio_path'] ?? '').toString().trim();
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
      'chat_history::${_userIdController.text.trim()}::$scopeKey';

  Future<void> _loadHistory(String scopeKey) async {
    final userId = _userIdController.text.trim();
    if (userId.isEmpty) {
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString(_historyStorageKey(scopeKey));
    if (raw == null || raw.isEmpty) {
      _historyByScope[scopeKey] = <ChatMessage>[];
      if (mounted) {
        setState(() {});
        if (scopeKey == _currentScopeKey) {
          _scrollToBottom(animated: false);
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
          _scrollToBottom(animated: false);
        }
      }
    } catch (_) {
      _historyByScope[scopeKey] = <ChatMessage>[];
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
    await prefs.setString(_historyStorageKey(scopeKey), jsonEncode(data));
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
    if (_sessionToken.isEmpty) {
      return;
    }
    try {
      final groups = await _client.listGroups();
      final previousGroupId = _currentGroupId;
      final previousTabsExpanded = _groupTabsExpanded;
      final nextGroupId = _resolvePreferredGroupId(
        groups,
        preferredGroupId: _currentGroupId,
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
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    try {
      final groups = await _client.mutateGroup(action, groupId);
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
      final resp = await _client.login();
      final sessionToken = (resp['session_token'] ?? '').toString();
      final obsAgentBaseUrl = (resp['obs_agent_base_url'] ?? '')
          .toString()
          .trim();
      if (sessionToken.isEmpty) {
        throw const FormatException('missing session_token');
      }
      if (mounted) {
        setState(() {
          _sessionToken = sessionToken;
          _obsAgentBaseUrl = obsAgentBaseUrl;
          _lastSequence = 0;
          _currentGroupId = '';
          _historyByScope.clear();
          _seenMessageIds.clear();
          _groups.clear();
          _status = 'Login success, connecting WebSocket...';
        });
      }
      await _loadHistory('direct');
      await _refreshGroups();
      unawaited(_connectWs());
      // Check for incomplete Vosk model download after login
      unawaited(_restoreVoskDownloadProgress());
    } catch (err) {
      if (mounted) {
        setState(() {
          _sessionToken = '';
          _obsAgentBaseUrl = '';
          _status = 'Login failed';
        });
      }
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
    if (_sessionToken.isEmpty) {
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
      final socket = await _client.connectWebSocket();
      await _socketSub?.cancel();
      await _socket?.close();

      _socket = socket;
      _socketSub = socket.listen(
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
    } catch (err) {
      final errorText = _describeRequestError(
        err,
        operation: 'WebSocket connect',
      );
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
    await _socket?.close();
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
        if (_seenMessageIds.contains(envelope.messageId)) {
          return;
        }
      }
      final meta = envelope.meta ?? <String, dynamic>{};
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

      if (_shouldFilterIncomingEnvelope(
        envelope: envelope,
        meta: meta,
        isGroupMessage: isGroupMessage,
      )) {
        return;
      }

      final when = DateTime.fromMillisecondsSinceEpoch(envelope.timestamp);
      final resolvedMeta = await _hydrateIncomingMediaMeta(
        messageType: envelope.messageType,
        meta: meta,
      );
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

      _appendMessage(
        ChatMessage(
          content: envelope.content,
          direction: direction,
          timestamp: when,
          scopeKey: scopeKey,
          authorId: fromUser,
          groupId: groupId,
          messageType: envelope.messageType,
          meta: resolvedMeta,
        ),
        updateStatus: isSystemMessage ? envelope.content : 'Received message',
      );
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
          meta: resolvedMeta,
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

  void _sendSocketAck(String messageId) {
    final socket = _socket;
    if (socket == null || messageId.trim().isEmpty) {
      return;
    }
    try {
      socket.add(
        jsonEncode(<String, dynamic>{'type': 'ack', 'message_id': messageId}),
      );
    } catch (_) {}
  }

  void _handleSocketClosed(String text) {
    _socketSub = null;
    _socket = null;
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

    if (isGroupMessage) {
      return messageType == 'system';
    }

    if (messageType == 'system') {
      return true;
    }

    if (origin == 'llm-agent' && _looksLikeStatusMessage(content)) {
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
            await _client.downloadAttachmentToFile(
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
          }
          _clearDownloadStatus(successText: '语音下载完成');
          resolved['audio_path'] = audioPath;
          return resolved;
        case 'image':
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
            await _client.downloadAttachmentToFile(
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
          }
          _clearDownloadStatus(successText: '图片下载完成');
          final bytes = await imageFile.readAsBytes();
          resolved['image_base64'] = base64Encode(bytes);
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
            await _client.downloadAttachmentToFile(
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
      await _client.downloadAttachmentToFile(
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
    await _persistHistory(target.scopeKey);
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
    if (!Platform.isAndroid) {
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

  bool get _composerHasText => _messageController.text.trim().isNotEmpty;

  void _toggleVoiceInputMode() {
    if (_recording || _sending || _transcribingVoice) {
      return;
    }
    final nextMode = !_voiceInputMode;
    setState(() {
      _voiceInputMode = nextMode;
    });
    if (nextMode) {
      FocusScope.of(context).unfocus();
      return;
    }
    _messageFocusNode.requestFocus();
  }

  void _focusTextComposer() {
    if (_recording || _transcribingVoice) {
      return;
    }
    if (_voiceInputMode) {
      setState(() {
        _voiceInputMode = false;
      });
    }
    _messageFocusNode.requestFocus();
  }

  VoiceGestureAction get _currentVoiceGestureAction =>
      resolveVoiceGestureAction(_recordDragOffset);

  Offset _resolveVoiceDragOffset({
    Offset? globalPosition,
    Offset? fallbackOffset,
  }) {
    final origin = _recordDragStartGlobalPosition;
    if (origin != null && globalPosition != null) {
      return globalPosition - origin;
    }
    return fallbackOffset ?? _recordDragOffset;
  }

  Future<void> _handleVoiceStart(LongPressStartDetails details) async {
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_recording || _sending) {
      return;
    }

    final hasPermission = await _audioRecorder.hasPermission();
    if (!hasPermission) {
      _appendSystem('Microphone permission denied.');
      return;
    }

    try {
      final tempDir = await getTemporaryDirectory();
      final useWaveFile = Platform.isWindows || _useLocalVosk;
      final fileExt = useWaveFile ? 'wav' : 'm4a';
      final path =
          '${tempDir.path}${Platform.pathSeparator}app_voice_${DateTime.now().millisecondsSinceEpoch}.$fileExt';
      await _audioRecorder.start(
        RecordConfig(
          encoder: useWaveFile ? AudioEncoder.wav : AudioEncoder.aacLc,
          bitRate: useWaveFile ? 256000 : 64000,
          sampleRate: useWaveFile ? 16000 : 16000,
          numChannels: 1,
        ),
        path: path,
      );
      if (_speechReady && !_useLocalVosk) {
        try {
          // Check if locale is available
          final locales = await _speechToText.locales();
          final zhLocale = locales.firstWhere(
            (l) => l.localeId == 'zh_CN' || l.localeId.startsWith('zh'),
            orElse: () => locales.first,
          );
          _appendSystem(
            'Using speech locale: ${zhLocale.name} (${zhLocale.localeId})',
          );
        } catch (e) {
          _appendSystem('Get locales failed: $e');
        }

        try {
          final listenResult = await _speechToText.listen(
            onResult: (result) {
              if (!mounted) {
                return;
              }
              final words = normalizeSpeechTranscript(result.recognizedWords);
              if (words.isNotEmpty) {
                _appendSystem('Recognized: $words');
              }
              setState(() {
                _speechDraft = words;
              });
            },
            onSoundLevelChange: (level) {
              // Sound level changes - useful for debugging
              if (!mounted) return;
              debugPrint('Sound level: $level');
            },
            pauseFor: const Duration(seconds: 2),
            listenFor: const Duration(minutes: 1),
            localeId: 'zh_CN',
            listenOptions: stt.SpeechListenOptions(
              listenMode: stt.ListenMode.dictation,
              partialResults: true,
              cancelOnError: false,
            ),
          );
          _appendSystem('Speech listen started: $listenResult');
        } catch (e, stack) {
          _appendSystem('Speech listen failed: $e');
          debugPrint('Speech listen error: $e\n$stack');
        }
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _recording = true;
        _recordDragOffset = Offset.zero;
        _recordDragStartGlobalPosition = details.globalPosition;
        _speechDraft = '';
        _recordStartedAt = DateTime.now();
        _status = 'Recording...';
      });
    } catch (err) {
      _appendSystem('Voice record start failed: $err');
    }
  }

  void _handleVoiceMove(LongPressMoveUpdateDetails details) {
    if (!_recording) {
      return;
    }
    setState(() {
      _recordDragOffset = _resolveVoiceDragOffset(
        globalPosition: details.globalPosition,
        fallbackOffset: details.offsetFromOrigin,
      );
    });
  }

  Future<void> _handleVoiceEnd(LongPressEndDetails details) async {
    if (!_recording) {
      return;
    }
    final dragOffset = _resolveVoiceDragOffset(
      globalPosition: details.globalPosition,
      fallbackOffset: _recordDragOffset,
    );
    if (mounted) {
      setState(() {
        _recordDragOffset = dragOffset;
      });
    } else {
      _recordDragOffset = dragOffset;
    }

    switch (resolveVoiceGestureAction(dragOffset)) {
      case VoiceGestureAction.cancel:
      await _cancelVoice();
      return;
      case VoiceGestureAction.transcribe:
      await _transcribeVoiceToDraft();
      return;
      case VoiceGestureAction.sendAudio:
        await _sendVoiceAsAudio();
    }
  }

  Future<RecordedAudio?> _stopRecording({required bool discard}) async {
    final startedAt = _recordStartedAt;
    _recordStartedAt = null;
    _recordDragStartGlobalPosition = null;
    try {
      if (_speechToText.isListening) {
        await _speechToText.stop();
      }
    } catch (_) {}

    String? path;
    try {
      path = await _audioRecorder.stop();
    } catch (_) {}

    final duration = startedAt == null
        ? Duration.zero
        : DateTime.now().difference(startedAt);

    if (!mounted) {
      return null;
    }
    setState(() {
      _recording = false;
      _recordDragOffset = Offset.zero;
    });

    if (path == null || path.isEmpty) {
      return null;
    }
    if (discard) {
      try {
        await File(path).delete();
      } catch (_) {}
      return null;
    }
    return RecordedAudio(path: path, duration: duration);
  }

  Future<void> _cancelVoice() async {
    try {
      if (_speechToText.isListening) {
        await _speechToText.cancel();
      }
    } catch (_) {}
    await _stopRecording(discard: true);
    _appendSystem('Voice input cancelled.');
  }

  Future<void> _transcribeVoiceToDraft() async {
    final recorded = await _stopRecording(discard: false);
    if (recorded == null) {
      _appendSystem('语音录制不可用。');
      return;
    }
    try {
      if (mounted) {
        setState(() {
          _transcribingVoice = true;
          _status = '语音转文字中...';
        });
      }

      var transcript = _speechDraft.trim();
      if (_useLocalVosk) {
        transcript = await _voskTranscriber.transcribeFile(recorded.path);
      }
      transcript = normalizeSpeechTranscript(transcript);

      if (transcript.isEmpty) {
        _appendSystem('未识别到有效语音内容，请重试。');
        return;
      }

      final existing = _messageController.text.trim();
      final merged = existing.isEmpty ? transcript : '$existing\n$transcript';
      _messageController.value = TextEditingValue(
        text: merged,
        selection: TextSelection.collapsed(offset: merged.length),
      );
      _speechDraft = transcript;
      if (mounted) {
        setState(() {
          _voiceInputMode = false;
          _status = '语音已转成文字，可修改后发送';
        });
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) {
            return;
          }
          _messageFocusNode.requestFocus();
        });
      }
    } catch (err) {
      _appendSystem('本地语音识别失败：$err');
    } finally {
      try {
        await File(recorded.path).delete();
      } catch (_) {}
      if (mounted) {
        setState(() {
          _transcribingVoice = false;
        });
      }
    }
  }

  Future<void> _sendVoiceAsAudio() async {
    final recorded = await _stopRecording(discard: false);
    if (recorded == null) {
      _appendSystem('Voice recording unavailable.');
      return;
    }

    try {
      final file = File(recorded.path);
      final bytes = await file.readAsBytes();
      await file.delete();
      if (bytes.length > 768 * 1024) {
        _appendSystem('Voice message too large. Please keep it shorter.');
        return;
      }

      final seconds = recorded.duration.inMilliseconds / 1000;
      final label = '[Voice ${seconds.toStringAsFixed(1)}s]';
      final audioFormat = (Platform.isWindows || _useLocalVosk) ? 'wav' : 'm4a';
      final savedAudioPath = await _persistVoiceMessage(
        bytes: bytes,
        extension: audioFormat,
      );
      _appendOutgoing(
        label,
        messageType: 'audio',
        meta: <String, dynamic>{
          'audio_path': savedAudioPath,
          'audio_format': audioFormat,
          'duration_ms': recorded.duration.inMilliseconds,
          if (_speechDraft.trim().isNotEmpty)
            'speech_text': _speechDraft.trim(),
          'input_mode': 'voice_audio',
          if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
          if (_currentGroupId.isNotEmpty) 'scope': 'group',
        },
      );
      setState(() {
        _sending = true;
      });
      try {
        await _client.sendAppMessage(
          label,
          messageType: 'audio',
          meta: <String, dynamic>{
            'audio_base64': base64Encode(bytes),
            'audio_format': audioFormat,
            'duration_ms': recorded.duration.inMilliseconds,
            if (_speechDraft.trim().isNotEmpty)
              'speech_text': _speechDraft.trim(),
            'input_mode': 'voice_audio',
            if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
            if (_currentGroupId.isNotEmpty) 'scope': 'group',
          },
        );
        if (mounted) {
          setState(() {
            _status = 'Voice audio sent';
          });
        }
      } finally {
        if (mounted) {
          setState(() {
            _sending = false;
          });
        }
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send voice audio'));
    }
  }

  Future<void> _handleAttachmentMenuAction(_AttachmentMenuAction action) async {
    switch (action) {
      case _AttachmentMenuAction.galleryImage:
        return _pickAndSendImage(ImageSource.gallery);
      case _AttachmentMenuAction.cameraImage:
        return _pickAndSendImage(ImageSource.camera);
    }
  }

  Future<void> _pickAndSendImage(ImageSource source) async {
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_sending || _recording) {
      return;
    }

    try {
      final picked = await _imagePicker.pickImage(
        source: source,
        imageQuality: 92,
      );
      if (picked == null) {
        return;
      }

      final bytes = await picked.readAsBytes();
      if (bytes.isEmpty) {
        _appendSystem('Selected image is empty.');
        return;
      }
      if (bytes.length > 4 * 1024 * 1024) {
        _appendSystem('Image too large. Please choose one under 4 MB.');
        return;
      }

      final fileName = picked.name.trim().isEmpty
          ? 'image_${DateTime.now().millisecondsSinceEpoch}.jpg'
          : picked.name.trim();
      final imageFormat = _detectImageFormat(fileName, bytes);
      final imageBase64 = base64Encode(bytes);
      final localMeta = <String, dynamic>{
        'image_base64': imageBase64,
        'image_format': imageFormat,
        'file_name': fileName,
        'input_mode': source == ImageSource.camera
            ? 'camera_image'
            : 'gallery_image',
        if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
        if (_currentGroupId.isNotEmpty) 'scope': 'group',
      };

      _appendOutgoing('', messageType: 'image', meta: localMeta);
      setState(() {
        _sending = true;
      });

      try {
        await _client.sendAppMessage('', messageType: 'image', meta: localMeta);
        if (mounted) {
          setState(() {
            _status = 'Image sent';
          });
        }
      } finally {
        if (mounted) {
          setState(() {
            _sending = false;
          });
        }
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send image'));
    }
  }

  Future<String> _persistVoiceMessage({
    required List<int> bytes,
    required String extension,
  }) async {
    return _persistAttachmentBytes(
      bytes: bytes,
      subdir: 'voice_messages',
      prefix: 'voice',
      extension: extension,
    );
  }

  Future<String> _persistAttachmentBytes({
    required List<int> bytes,
    required String subdir,
    required String prefix,
    required String extension,
  }) async {
    final supportDir = await getApplicationSupportDirectory();
    final voiceDir = Directory(
      '${supportDir.path}${Platform.pathSeparator}$subdir',
    );
    if (!await voiceDir.exists()) {
      await voiceDir.create(recursive: true);
    }
    final file = File(
      '${voiceDir.path}${Platform.pathSeparator}${prefix}_${DateTime.now().millisecondsSinceEpoch}.$extension',
    );
    await file.writeAsBytes(bytes, flush: true);
    return file.path;
  }

  Future<String> _attachmentPathForFileID({
    required String fileId,
    required String subdir,
    required String prefix,
    required String extension,
  }) async {
    final supportDir = await getApplicationSupportDirectory();
    final targetDir = Directory(
      '${supportDir.path}${Platform.pathSeparator}$subdir',
    );
    if (!await targetDir.exists()) {
      await targetDir.create(recursive: true);
    }
    final safeFileID = fileId
        .replaceAll(RegExp(r'[^A-Za-z0-9._-]'), '_')
        .replaceAll('__', '_');
    final ext = extension.trim().isEmpty
        ? 'bin'
        : extension.trim().toLowerCase();
    return '${targetDir.path}${Platform.pathSeparator}${prefix}_$safeFileID.$ext';
  }

  void _updateDownloadStatus({
    required String label,
    required int receivedBytes,
    required int? totalBytes,
    required bool resumed,
  }) {
    final percent = totalBytes == null || totalBytes <= 0
        ? -1
        : ((receivedBytes * 100) / totalBytes).floor().clamp(0, 100);
    if (!mounted) {
      return;
    }
    if (_downloadStatusLabel == label && _downloadStatusPercent == percent) {
      return;
    }
    final progressText = totalBytes == null || totalBytes <= 0
        ? _formatBytes(receivedBytes)
        : '${_formatBytes(receivedBytes)} / ${_formatBytes(totalBytes)}';
    final resumeText = resumed ? '继续下载' : '下载中';
    setState(() {
      _downloadStatusLabel = label;
      _downloadStatusPercent = percent;
      _status = percent >= 0
          ? '$resumeText $label $percent% ($progressText)'
          : '$resumeText $label ($progressText)';
    });
  }

  void _clearDownloadStatus({String? successText}) {
    if (!mounted) {
      return;
    }
    setState(() {
      _downloadStatusLabel = null;
      _downloadStatusPercent = -1;
      if (successText != null && successText.trim().isNotEmpty) {
        _status = successText;
      }
    });
  }

  String _formatBytes(int bytes) {
    if (bytes < 1024) {
      return '$bytes B';
    }
    final kb = bytes / 1024;
    if (kb < 1024) {
      return '${kb.toStringAsFixed(kb >= 100 ? 0 : 1)} KB';
    }
    final mb = kb / 1024;
    if (mb < 1024) {
      return '${mb.toStringAsFixed(mb >= 100 ? 0 : 1)} MB';
    }
    final gb = mb / 1024;
    return '${gb.toStringAsFixed(gb >= 100 ? 0 : 1)} GB';
  }

  String _detectImageFormat(String fileName, List<int> bytes) {
    final lowerName = fileName.toLowerCase();
    if (lowerName.endsWith('.png')) {
      return 'png';
    }
    if (lowerName.endsWith('.webp')) {
      return 'webp';
    }
    if (lowerName.endsWith('.gif')) {
      return 'gif';
    }
    if (lowerName.endsWith('.bmp')) {
      return 'bmp';
    }
    if (bytes.length >= 4 &&
        bytes[0] == 0x89 &&
        bytes[1] == 0x50 &&
        bytes[2] == 0x4E &&
        bytes[3] == 0x47) {
      return 'png';
    }
    return 'jpg';
  }

  Future<void> _sendMessage() async {
    if (_sessionToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    final text = _messageController.text.trim();
    if (text.isEmpty) {
      return;
    }
    FocusScope.of(context).unfocus();
    _messageController.clear();
    _appendOutgoing(text);
    setState(() {
      _sending = true;
    });
    try {
      if (_currentGroupId.isEmpty) {
        await _client.sendMessage(text);
      } else {
        await _client.sendAppMessage(
          text,
          meta: <String, dynamic>{
            'group_id': _currentGroupId,
            'scope': 'group',
          },
        );
      }
      if (mounted) {
        setState(() {
          _status = 'Message sent';
        });
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send message'));
    } finally {
      if (mounted) {
        setState(() {
          _sending = false;
        });
      }
    }
  }

  AppPalette get _palette => context.appPalette;

  Color get _connectionColor {
    final palette = _palette;
    if (_connected) {
      return palette.success;
    }
    if (_connecting || _loggingIn) {
      return palette.warning;
    }
    return palette.error;
  }

  String get _connectionLabel {
    if (_connected) {
      return 'Connected';
    }
    if (_connecting) {
      return 'Connecting';
    }
    if (_sessionToken.isEmpty) {
      return 'Login required';
    }
    return 'Offline';
  }

  Widget _buildStatusChip({
    required IconData icon,
    required String label,
    required Color color,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: color.withValues(alpha: 0.18)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              color: color,
              fontWeight: FontWeight.w700,
              fontSize: 12,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildConfigItem({
    required IconData icon,
    required String label,
    required String value,
    required VoidCallback? onCopy,
  }) {
    final palette = _palette;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: palette.surfaceRaised,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: palette.border),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: palette.textSecondary),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    color: palette.textMuted,
                  ),
                ),
                const SizedBox(height: 4),
                SelectionArea(
                  child: Text(
                    value.isEmpty ? '-' : value,
                    style: TextStyle(
                      fontSize: 13,
                      height: 1.35,
                      color: palette.textPrimary,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            onPressed: onCopy,
            tooltip: 'Copy $label',
            visualDensity: VisualDensity.compact,
            icon: const Icon(Icons.copy_rounded, size: 18),
          ),
        ],
      ),
    );
  }

  Widget _buildVoskModelCard() {
    final palette = _palette;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: palette.surfaceRaised,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: palette.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.model_training_rounded,
                size: 18,
                color: palette.accent,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Vosk 语音模型',
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                        color: palette.textMuted,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '中文语音识别小模型（约 40MB）',
                      style: TextStyle(fontSize: 12, color: palette.textSecondary),
                    ),
                  ],
                ),
              ),
              FutureBuilder<bool>(
                key: const ValueKey('vosk_model_check'),
                future: _isVoskModelDownloaded(),
                builder: (context, snapshot) {
                  final isDownloaded = snapshot.data ?? false;
                  if (_voskModelDownloading) {
                    return Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: palette.surfaceSoft,
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(color: palette.borderStrong),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          SizedBox(
                            width: 14,
                            height: 14,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              value: _voskModelDownloadProgress,
                              backgroundColor: palette.surfaceMuted,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            '${(_voskModelDownloadProgress * 100).toStringAsFixed(0)}%',
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w700,
                              color: palette.textPrimary,
                            ),
                          ),
                        ],
                      ),
                    );
                  }
                  if (isDownloaded) {
                    return Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: palette.success.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(
                          color: palette.success.withValues(alpha: 0.3),
                        ),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.check_circle_rounded,
                            size: 14,
                            color: palette.success,
                          ),
                          const SizedBox(width: 6),
                          Text(
                            '已安装',
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w700,
                              color: palette.success,
                            ),
                          ),
                        ],
                      ),
                    );
                  }
                  return FutureBuilder<bool>(
                    key: const ValueKey('vosk_partial_check'),
                    future: _hasPartialVoskDownload(),
                    builder: (context, partialSnapshot) {
                      final hasPartial = partialSnapshot.data ?? false;
                      return FilledButton.tonal(
                        onPressed: _voskModelDownloading
                            ? null
                            : _downloadAndExtractVoskModel,
                        style: FilledButton.styleFrom(
                          backgroundColor:
                              hasPartial ? palette.warning : palette.accent,
                          foregroundColor: _foregroundForColor(
                            hasPartial ? palette.warning : palette.accent,
                          ),
                          minimumSize: const Size(0, 32),
                          padding: const EdgeInsets.symmetric(horizontal: 12),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12),
                          ),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            if (hasPartial) ...[
                              const Icon(Icons.play_arrow_rounded, size: 14),
                              const SizedBox(width: 4),
                            ],
                            Text(
                              hasPartial ? '继续下载' : '下载',
                              style: const TextStyle(fontSize: 12),
                            ),
                          ],
                        ),
                      );
                    },
                  );
                },
              ),
            ],
          ),
          if (_voskModelDownloadError != null) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: palette.error.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: palette.error.withValues(alpha: 0.3),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.error_outline_rounded,
                    size: 14,
                    color: palette.error,
                  ),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      _voskModelDownloadError!,
                      style: TextStyle(
                        fontSize: 11,
                        color: palette.error,
                      ),
                    ),
                  ),
                  IconButton(
                    onPressed: () {
                      setState(() {
                        _voskModelDownloadError = null;
                      });
                    },
                    tooltip: '关闭',
                    visualDensity: VisualDensity.compact,
                    iconSize: 16,
                    icon: Icon(
                      Icons.close_rounded,
                      size: 14,
                      color: palette.error,
                    ),
                  ),
                ],
              ),
            ),
          ],
          if (_voskModelDownloadError != null ||
              _hasPartialVoskDownloadSync()) ...[
            const SizedBox(height: 4),
            FutureBuilder<bool>(
              future: _hasPartialVoskDownload(),
              builder: (context, snapshot) {
                final hasPartial = snapshot.data ?? false;
                if (!hasPartial) return const SizedBox.shrink();
                return TextButton.icon(
                  onPressed: _clearVoskDownloadCache,
                  style: TextButton.styleFrom(
                    foregroundColor: palette.textSecondary,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 4,
                    ),
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  icon: const Icon(Icons.delete_outline_rounded, size: 12),
                  label: const Text('清除缓存', style: TextStyle(fontSize: 11)),
                );
              },
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildThemePresetCard() {
    final palette = _palette;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: palette.surfaceRaised,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: palette.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.palette_outlined, size: 18, color: palette.textSecondary),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'UI 主题色',
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                        color: palette.textMuted,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '切换整套 UI 的主色、背景和控件氛围',
                      style: TextStyle(fontSize: 12, color: palette.textSecondary),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final preset in kUiThemePresets)
                ChoiceChip(
                  selected: widget.themePreset.id == preset.id,
                  label: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Container(
                        width: 10,
                        height: 10,
                        decoration: BoxDecoration(
                          color: preset.accent,
                          borderRadius: BorderRadius.circular(999),
                        ),
                      ),
                      const SizedBox(width: 6),
                      Text(preset.label),
                    ],
                  ),
                  onSelected: (_) => widget.onThemePresetChanged(preset),
                ),
            ],
          ),
        ],
      ),
    );
  }

  bool _hasPartialVoskDownloadSync() {
    // Quick sync check for UI rendering
    return _voskModelDownloadProgress > 0 && _voskModelDownloadProgress < 1.0;
  }

  Future<void> _clearVoskDownloadCache() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(_voskDownloadProgressKey);
      await prefs.remove(_voskDownloadBytesKey);

      final archiveFile = await _getVoskArchiveFile();
      final partFile = await _getVoskArchivePartFile();
      final tempModelDir = await _getVoskExtractionTempDir();
      await _deleteFileIfExists(archiveFile);
      await _deleteFileIfExists(partFile);
      await _deleteDirectoryIfExists(tempModelDir);

      if (!mounted) {
        return;
      }
      setState(() {
        _voskModelDownloadProgress = 0.0;
        _voskModelDownloadError = null;
      });
      _appendSystem('已清除 Vosk 模型下载缓存');
    } catch (err) {
      _appendSystem('清除缓存失败: $err');
    }
  }

  Widget _buildAttachmentMenuButton({required bool enabled}) {
    final palette = _palette;
    return PopupMenuButton<_AttachmentMenuAction>(
      enabled: enabled,
      tooltip: 'Attachments',
      onSelected: (action) => unawaited(_handleAttachmentMenuAction(action)),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      color: palette.surfaceMuted,
      itemBuilder: (context) => [
        PopupMenuItem<_AttachmentMenuAction>(
          value: _AttachmentMenuAction.galleryImage,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(
              Icons.photo_library_rounded,
              color: palette.textPrimary,
            ),
            title: Text(
              'Send Image',
              style: TextStyle(color: palette.textPrimary),
            ),
            subtitle: Text(
              'Choose from gallery',
              style: TextStyle(color: palette.textSecondary),
            ),
          ),
        ),
        PopupMenuItem<_AttachmentMenuAction>(
          value: _AttachmentMenuAction.cameraImage,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(
              Icons.photo_camera_back_rounded,
              color: palette.textPrimary,
            ),
            title: Text(
              'Take Photo',
              style: TextStyle(color: palette.textPrimary),
            ),
            subtitle: Text(
              'Capture and send',
              style: TextStyle(color: palette.textSecondary),
            ),
          ),
        ),
      ],
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 160),
        height: 42,
        width: 42,
        decoration: BoxDecoration(
          color: enabled ? palette.surfaceSoft : palette.surfaceMuted,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(
            color: enabled ? palette.borderStrong : palette.border,
          ),
        ),
        child: Icon(
          Icons.add_rounded,
          color: enabled ? palette.textPrimary : palette.textMuted,
        ),
      ),
    );
  }

  Widget _buildComposerIconButton({
    required IconData icon,
    required VoidCallback? onPressed,
    bool active = false,
    Color? activeColor,
  }) {
    final palette = _palette;
    final enabled = onPressed != null;
    final effectiveActiveColor = activeColor ?? palette.accent;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onPressed,
        borderRadius: BorderRadius.circular(14),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 160),
          height: 42,
          width: 42,
          decoration: BoxDecoration(
            color: active
                ? effectiveActiveColor
                : enabled
                ? palette.surfaceSoft
                : palette.surfaceMuted,
            borderRadius: BorderRadius.circular(14),
            border: Border.all(
              color: active
                  ? effectiveActiveColor.withValues(alpha: 0.9)
                  : enabled
                  ? palette.borderStrong
                  : palette.border,
            ),
          ),
          child: Icon(
            icon,
            color: active
                ? _foregroundForColor(effectiveActiveColor)
                : enabled
                ? palette.textPrimary
                : palette.textMuted,
            size: 21,
          ),
        ),
      ),
    );
  }

  Widget _buildTopPanel() {
    final palette = _palette;
    final canLogin = !_loggingIn && !_configLoading && _clientConfig != null;
    final baseUrl = _clientConfig?.baseUrl ?? '';
    final receiveToken = _clientConfig?.receiveToken ?? '';
    final hasGroups = _groups.isNotEmpty;
    final groupsTabSelected = _currentGroupId.isNotEmpty;
    final showGroupTabs = hasGroups && _groups.length > 1 && _groupTabsExpanded;
    final controlsMaxHeight = math.min(
      MediaQuery.sizeOf(context).height * 0.52,
      440.0,
    );
    final compactButtonLabel = _controlsExpanded
        ? 'Hide'
        : (_sessionToken.isEmpty ? 'Login' : 'Controls');

    return Container(
      margin: const EdgeInsets.fromLTRB(10, 0, 10, 8),
      padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: <Color>[palette.surface, palette.surfaceRaised],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: palette.border),
        boxShadow: [
          BoxShadow(
            blurRadius: 18,
            color: Colors.black.withValues(alpha: 0.22),
            offset: const Offset(0, 10),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(
                      height: 44,
                      child: ListView(
                        scrollDirection: Axis.horizontal,
                        padding: const EdgeInsets.symmetric(vertical: 2),
                        children: [
                          ChoiceChip(
                            selected: !groupsTabSelected,
                            label: const Text('Direct'),
                            onSelected: (_) => unawaited(_switchToDirectScope()),
                          ),
                          if (hasGroups) ...[
                            const SizedBox(width: 8),
                            ChoiceChip(
                              selected: groupsTabSelected,
                              label: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Text(
                                    _groups.length == 1
                                        ? 'Group'
                                        : 'Groups (${_groups.length})',
                                  ),
                                  if (_groups.length > 1) ...[
                                    const SizedBox(width: 4),
                                    Icon(
                                      showGroupTabs
                                          ? Icons.expand_less_rounded
                                          : Icons.expand_more_rounded,
                                      size: 16,
                                    ),
                                  ],
                                ],
                              ),
                              onSelected: (_) => unawaited(_switchToGroupsTab()),
                            ),
                          ],
                        ],
                      ),
                    ),
                    if (showGroupTabs) ...[
                      const SizedBox(height: 8),
                      SizedBox(
                        height: 46,
                        child: ListView.separated(
                          scrollDirection: Axis.horizontal,
                          padding: const EdgeInsets.symmetric(vertical: 3),
                          itemCount: _groups.length,
                          separatorBuilder: (_, _) => const SizedBox(width: 8),
                          itemBuilder: (context, index) {
                            final group = _groups[index];
                            return ChoiceChip(
                              selected: _currentGroupId == group.id,
                              label: Text('${group.id} (${group.members.length})'),
                              onSelected: (_) =>
                                  unawaited(_switchToGroupScope(group.id)),
                            );
                          },
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: 8),
              OutlinedButton.icon(
                onPressed: () {
                  setState(() {
                    _controlsExpanded = !_controlsExpanded;
                  });
                },
                style: OutlinedButton.styleFrom(
                  foregroundColor: palette.textPrimary,
                  side: BorderSide(color: palette.borderStrong),
                  minimumSize: const Size(0, 40),
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                  ),
                ),
                icon: Icon(
                  _controlsExpanded
                      ? Icons.expand_less_rounded
                      : Icons.tune_rounded,
                ),
                label: Text(compactButtonLabel),
              ),
            ],
          ),
          AnimatedCrossFade(
            firstChild: const SizedBox.shrink(),
            secondChild: Padding(
              padding: const EdgeInsets.only(top: 10),
              child: ConstrainedBox(
                constraints: BoxConstraints(maxHeight: controlsMaxHeight),
                child: Scrollbar(
                  controller: _controlsScrollController,
                  thumbVisibility: true,
                  radius: const Radius.circular(999),
                  child: SingleChildScrollView(
                    controller: _controlsScrollController,
                    padding: const EdgeInsets.only(right: 6, bottom: 4),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        TextField(
                          controller: _userIdController,
                          decoration: const InputDecoration(
                            labelText: 'User ID',
                            hintText: 'demo-user',
                            prefixIcon: Icon(Icons.badge_outlined),
                            isDense: true,
                          ),
                        ),
                        const SizedBox(height: 10),
                        TextField(
                          controller: _passwordController,
                          obscureText: !_passwordVisible,
                          decoration: InputDecoration(
                            labelText: 'Password',
                            hintText: 'blog-agent password',
                            prefixIcon: const Icon(Icons.lock_outline_rounded),
                            isDense: true,
                            suffixIcon: IconButton(
                              onPressed: () {
                                setState(() {
                                  _passwordVisible = !_passwordVisible;
                                });
                              },
                              icon: Icon(
                                _passwordVisible
                                    ? Icons.visibility_off_outlined
                                    : Icons.visibility_outlined,
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(height: 10),
                        TextField(
                          controller: _groupIdController,
                          decoration: const InputDecoration(
                            labelText: 'Group ID',
                            hintText: 'party-01',
                            prefixIcon: Icon(Icons.groups_2_outlined),
                            isDense: true,
                          ),
                        ),
                        const SizedBox(height: 12),
                        Wrap(
                          spacing: 10,
                          runSpacing: 10,
                          children: [
                            FilledButton.icon(
                              onPressed: canLogin ? _login : null,
                              style: FilledButton.styleFrom(
                                backgroundColor: palette.accent,
                                foregroundColor: _foregroundForColor(
                                  palette.accent,
                                ),
                                minimumSize: const Size(120, 48),
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 14,
                                ),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(18),
                                ),
                              ),
                              icon: Icon(
                                _sessionToken.isEmpty
                                    ? Icons.login
                                    : Icons.refresh_rounded,
                              ),
                              label: Text(
                                _sessionToken.isEmpty ? 'Login' : 'Re-login',
                              ),
                            ),
                            OutlinedButton.icon(
                              onPressed: _connected || _connecting
                                  ? _disconnectWs
                                  : null,
                              style: OutlinedButton.styleFrom(
                                foregroundColor: palette.textPrimary,
                                minimumSize: const Size(120, 48),
                                side: BorderSide(color: palette.borderStrong),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(18),
                                ),
                              ),
                              icon: const Icon(Icons.link_off_rounded),
                              label: const Text('Disconnect'),
                            ),
                          ],
                        ),
                        const SizedBox(height: 10),
                        Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: [
                            FilledButton.tonal(
                              onPressed: _sessionToken.isEmpty
                                  ? null
                                  : () => _mutateGroup('create'),
                              child: const Text('Create'),
                            ),
                            FilledButton.tonal(
                              onPressed: _sessionToken.isEmpty
                                  ? null
                                  : () => _mutateGroup('join'),
                              child: const Text('Join'),
                            ),
                            OutlinedButton(
                              onPressed: _sessionToken.isEmpty
                                  ? null
                                  : () => _mutateGroup('leave'),
                              child: const Text('Leave'),
                            ),
                          ],
                        ),
                        const SizedBox(height: 12),
                        _buildConfigItem(
                          icon: Icons.link_rounded,
                          label: 'Server URL',
                          value: baseUrl,
                          onCopy: baseUrl.isEmpty
                              ? null
                              : () => unawaited(_copyText('URL', baseUrl)),
                        ),
                        const SizedBox(height: 8),
                        _buildConfigItem(
                          icon: Icons.key_rounded,
                          label: 'Receive Token',
                          value: receiveToken,
                          onCopy: receiveToken.isEmpty
                              ? null
                              : () => unawaited(_copyText('Token', receiveToken)),
                        ),
                        const SizedBox(height: 8),
                        Row(
                          children: [
                            Expanded(
                              child: TextField(
                                controller: _baseUrlController,
                                keyboardType: TextInputType.url,
                                decoration: const InputDecoration(
                                  labelText: 'Server URL',
                                  hintText: 'http://127.0.0.1:9002',
                                  prefixIcon: Icon(Icons.link_rounded),
                                  isDense: true,
                                ),
                              ),
                            ),
                            const SizedBox(width: 10),
                            FilledButton.icon(
                              onPressed: _configLoading ? null : _saveBaseUrl,
                              style: FilledButton.styleFrom(
                                backgroundColor: palette.accent,
                                foregroundColor: _foregroundForColor(
                                  palette.accent,
                                ),
                                minimumSize: const Size(0, 48),
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 14,
                                ),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(18),
                                ),
                              ),
                              icon: const Icon(Icons.save_outlined),
                              label: const Text('Save URL'),
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        _buildVoskModelCard(),
                        const SizedBox(height: 8),
                        _buildThemePresetCard(),
                      ],
                    ),
                  ),
                ),
              ),
            ),
            crossFadeState: _controlsExpanded
                ? CrossFadeState.showSecond
                : CrossFadeState.showFirst,
            duration: const Duration(milliseconds: 180),
          ),
        ],
      ),
    );
  }

  Widget _buildComposer() {
    final palette = _palette;
    final canInteract = !(_sending || _recording || _transcribingVoice);
    final showSendButton = !_voiceInputMode && _composerHasText;

    return Container(
      margin: const EdgeInsets.fromLTRB(10, 0, 10, 10),
      padding: const EdgeInsets.fromLTRB(10, 10, 10, 10),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: <Color>[palette.surface, palette.surfaceMuted],
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
        ),
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: palette.border),
        boxShadow: [
          BoxShadow(
            blurRadius: 24,
            color: Colors.black.withValues(alpha: 0.18),
            offset: const Offset(0, 12),
          ),
        ],
      ),
      child: Column(
        children: [
          Container(
            width: double.infinity,
            padding: const EdgeInsets.fromLTRB(4, 0, 4, 8),
            child: Text(
              _status,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                color: palette.textMuted,
                fontWeight: FontWeight.w600,
                fontSize: 11,
              ),
            ),
          ),
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              _buildComposerIconButton(
                icon: _voiceInputMode
                    ? Icons.keyboard_alt_rounded
                    : Icons.graphic_eq_rounded,
                  onPressed: canInteract ? _toggleVoiceInputMode : null,
                  active: _voiceInputMode,
                  activeColor: palette.accent,
                ),
              const SizedBox(width: 10),
              Expanded(
                child: AnimatedSwitcher(
                  duration: const Duration(milliseconds: 180),
                  switchInCurve: Curves.easeOutCubic,
                  switchOutCurve: Curves.easeInCubic,
                  child: _voiceInputMode
                      ? GestureDetector(
                          key: const ValueKey('voice_composer'),
                          onLongPressStart: _handleVoiceStart,
                          onLongPressMoveUpdate: _handleVoiceMove,
                          onLongPressEnd: _handleVoiceEnd,
                          child: AnimatedContainer(
                            duration: const Duration(milliseconds: 160),
                            height: 42,
                            decoration: BoxDecoration(
                              color: _recording
                                  ? palette.accent
                                  : palette.surfaceSoft,
                              borderRadius: BorderRadius.circular(12),
                              border: Border.all(
                                color: _recording
                                    ? palette.accent.withValues(alpha: 0.95)
                                    : palette.borderStrong,
                              ),
                            ),
                            child: Center(
                              child: Text(
                                _recording ? '松手 发送' : '按住 说话',
                                style: TextStyle(
                                  color: (_recording
                                          ? _foregroundForColor(palette.accent)
                                          : palette.textPrimary)
                                      .withValues(alpha: _recording ? 1 : 0.9),
                                  fontSize: 16,
                                  fontWeight: FontWeight.w700,
                                  letterSpacing: 0.2,
                                ),
                              ),
                            ),
                          ),
                        )
                      : TextField(
                          key: const ValueKey('text_composer'),
                          controller: _messageController,
                          focusNode: _messageFocusNode,
                          minLines: 1,
                          maxLines: 3,
                          enabled: !_recording && !_transcribingVoice,
                          onChanged: (_) {
                            if (!mounted) {
                              return;
                            }
                            setState(() {});
                          },
                          onTap: () {
                            if (_voiceInputMode) {
                              setState(() {
                                _voiceInputMode = false;
                              });
                            }
                          },
                          onSubmitted: (_) => _sendMessage(),
                          style: TextStyle(
                            color: palette.textPrimary,
                            fontSize: 15,
                            height: 1.25,
                          ),
                          cursorColor: palette.accent,
                          decoration: InputDecoration(
                            hintText: _currentGroupId.isEmpty
                                ? '发消息'
                                : '发群消息',
                            hintStyle: TextStyle(
                              color: palette.textMuted,
                              fontSize: 15,
                            ),
                            filled: true,
                            fillColor: palette.surfaceSoft,
                            floatingLabelBehavior:
                                FloatingLabelBehavior.never,
                            isDense: true,
                            contentPadding: const EdgeInsets.symmetric(
                              horizontal: 14,
                              vertical: 10,
                            ),
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                              borderSide: BorderSide(color: palette.borderStrong),
                            ),
                            enabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                              borderSide: BorderSide(color: palette.borderStrong),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                              borderSide: BorderSide(color: palette.accent),
                            ),
                          ),
                        ),
                ),
              ),
              const SizedBox(width: 10),
              _buildComposerIconButton(
                icon: Icons.sentiment_satisfied_alt_rounded,
                onPressed: canInteract ? _focusTextComposer : null,
              ),
              const SizedBox(width: 10),
              if (showSendButton)
                _buildComposerIconButton(
                  icon: Icons.arrow_upward_rounded,
                  onPressed: canInteract ? _sendMessage : null,
                  active: true,
                  activeColor: palette.accent,
                )
              else
                _buildAttachmentMenuButton(enabled: canInteract),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildVoiceGestureOverlay() {
    final palette = _palette;
    final activeAction = _currentVoiceGestureAction;
    final showDraft = _speechDraft.trim().isNotEmpty;

    return IgnorePointer(
      ignoring: true,
      child: Stack(
        children: [
          Positioned.fill(
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: Colors.black.withValues(alpha: 0.52),
              ),
            ),
          ),
          Positioned.fill(
            child: SafeArea(
              child: Column(
                children: [
                  const Spacer(flex: 3),
                  Column(
                    children: [
                      Stack(
                        clipBehavior: Clip.none,
                        alignment: Alignment.center,
                        children: [
                          Container(
                            width: 142,
                            padding: const EdgeInsets.symmetric(
                              horizontal: 18,
                              vertical: 18,
                            ),
                            decoration: BoxDecoration(
                              color: palette.accent,
                              borderRadius: BorderRadius.circular(24),
                              boxShadow: [
                                BoxShadow(
                                  blurRadius: 32,
                                  color: palette.accent.withValues(alpha: 0.28),
                                  offset: const Offset(0, 16),
                                ),
                              ],
                            ),
                            child: const _VoiceWaveformBadge(),
                          ),
                          Positioned(
                            bottom: -8,
                            child: Transform.rotate(
                              angle: 0.78,
                              child: Container(
                                width: 16,
                                height: 16,
                                decoration: BoxDecoration(
                                  color: palette.accent,
                                  borderRadius: const BorderRadius.all(
                                    Radius.circular(4),
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 22),
                      Text(
                        showDraft ? _speechDraft.trim() : '正在聆听你的语音…',
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 15,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                  const Spacer(flex: 2),
                  SizedBox(
                    height: 246,
                    child: Stack(
                      clipBehavior: Clip.none,
                      children: [
                        Positioned(
                          left: -58,
                          right: -58,
                          bottom: -156,
                          child: Container(
                            height: 320,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              gradient: LinearGradient(
                                colors: [
                                  _blendWithAccent(
                                    palette.surfaceRaised,
                                    palette.accent,
                                    0.20,
                                  ),
                                  _blendWithAccent(
                                    palette.surface,
                                    palette.accent,
                                    0.08,
                                  ),
                                ],
                                begin: Alignment.topCenter,
                                end: Alignment.bottomCenter,
                              ),
                              border: Border.all(
                                color: palette.borderStrong.withValues(
                                  alpha: 0.7,
                                ),
                              ),
                              boxShadow: [
                                BoxShadow(
                                  blurRadius: 28,
                                  color: palette.accent.withValues(alpha: 0.16),
                                  offset: const Offset(0, -12),
                                ),
                              ],
                            ),
                            child: Padding(
                              padding: const EdgeInsets.only(top: 38),
                              child: Column(
                                children: [
                                  Text(
                                    activeAction == VoiceGestureAction.sendAudio
                                        ? '松手 发送'
                                        : activeAction ==
                                              VoiceGestureAction.cancel
                                        ? '松手 取消'
                                        : '松手 转文字',
                                    style: TextStyle(
                                      color: palette.textPrimary,
                                      fontSize: 28,
                                      fontWeight: FontWeight.w700,
                                    ),
                                  ),
                                  const SizedBox(height: 6),
                                  Text(
                                    '向左右滑动可切换操作',
                                    style: TextStyle(
                                      color: palette.textSecondary,
                                      fontSize: 13,
                                      fontWeight: FontWeight.w500,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                        Positioned(
                          left: 14,
                          bottom: 74,
                          child: Transform.rotate(
                            angle: -0.16,
                            child: _VoiceActionPill(
                              label: '取消',
                              active: activeAction == VoiceGestureAction.cancel,
                              alignment: Alignment.centerLeft,
                            ),
                          ),
                        ),
                        Positioned(
                          right: 14,
                          bottom: 74,
                          child: Transform.rotate(
                            angle: 0.16,
                            child: _VoiceActionPill(
                              label: '滑到这里 转文字',
                              active:
                                  activeAction == VoiceGestureAction.transcribe,
                              alignment: Alignment.centerRight,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final palette = _palette;
    return Scaffold(
      extendBodyBehindAppBar: true,
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Text('App Agent'),
                const SizedBox(width: 8),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 6,
                    vertical: 2,
                  ),
                  decoration: BoxDecoration(
                    color: palette.surfaceRaised.withValues(alpha: 0.9),
                    borderRadius: BorderRadius.circular(4),
                    border: Border.all(color: palette.border),
                  ),
                  child: Text(
                    appVersion,
                    style: TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.normal,
                      color: palette.textSecondary,
                    ),
                  ),
                ),
              ],
            ),
            Text(
              _currentGroupId.isEmpty
                  ? 'Direct conversation'
                  : 'Group ${_currentGroupId.toLowerCase()}',
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w500,
                color: palette.textSecondary,
              ),
            ),
          ],
        ),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: 16),
            child: Center(
              child: _buildStatusChip(
                icon: Icons.wifi_tethering_rounded,
                label: _connectionLabel,
                color: _connectionColor,
              ),
            ),
          ),
        ],
      ),
      body: Stack(
        children: [
          Container(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                colors: <Color>[
                  palette.backgroundTop,
                  palette.surfaceMuted,
                  palette.backgroundBottom,
                ],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              ),
            ),
            child: SafeArea(
              child: Column(
                children: [
                  _buildTopPanel(),
                  Expanded(
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(10, 0, 10, 8),
                      child: Container(
                        decoration: BoxDecoration(
                          color: palette.surface.withValues(alpha: 0.96),
                          borderRadius: BorderRadius.circular(24),
                          border: Border.all(color: palette.border),
                          boxShadow: [
                            BoxShadow(
                              blurRadius: 18,
                              color: Colors.black.withValues(alpha: 0.24),
                              offset: const Offset(0, 10),
                            ),
                          ],
                        ),
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(24),
                          child: ListView.builder(
                            controller: _scrollController,
                            padding: const EdgeInsets.fromLTRB(14, 14, 14, 14),
                            itemCount: _messages.length,
                            itemBuilder: (context, index) {
                              final msg = _messages[index];
                              return _MessageBubble(
                                message: msg,
                                isPlaying:
                                    _playingAudioKey == _messagePlaybackKey(msg),
                                onTap: () => _handleMessageTap(msg),
                                onCopy: () async {
                                  await Clipboard.setData(
                                    ClipboardData(text: msg.content),
                                  );
                                  if (!context.mounted) {
                                    return;
                                  }
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    const SnackBar(
                                      content: Text('Message copied'),
                                      duration: Duration(seconds: 1),
                                    ),
                                  );
                                },
                              );
                            },
                          ),
                        ),
                      ),
                    ),
                  ),
                  _buildComposer(),
                ],
              ),
            ),
          ),
          if (_recording) Positioned.fill(child: _buildVoiceGestureOverlay()),
        ],
      ),
    );
  }
}

class _MessageBubble extends StatelessWidget {
  const _MessageBubble({
    required this.message,
    required this.onTap,
    required this.onCopy,
    this.isPlaying = false,
  });

  final ChatMessage message;
  final Future<void> Function() onTap;
  final Future<void> Function() onCopy;
  final bool isPlaying;

  @override
  Widget build(BuildContext context) {
    final palette = context.appPalette;
    final accentForeground = _foregroundForColor(palette.accent);
    final bubbleMaxWidth = (MediaQuery.sizeOf(context).width * 0.9).clamp(
      280.0,
      980.0,
    );
    final isOutgoing = message.direction == MessageDirection.outgoing;
    final isSystem = message.direction == MessageDirection.system;
    final authorLabel = message.authorId.trim();
    final showAuthor = !isSystem && message.groupId.trim().isNotEmpty;
    final alignment = isSystem
        ? Alignment.center
        : (isOutgoing ? Alignment.centerRight : Alignment.centerLeft);
    final bgColor = isSystem
        ? palette.messageSystem
        : (isOutgoing ? palette.messageOutgoing : palette.messageIncoming);
    final fgColor = isOutgoing ? _foregroundForColor(bgColor) : palette.textPrimary;
    final isAudio = message.messageType == 'audio';
    final isImage = message.messageType == 'image';
    final isApk = isApkChatMessage(message);
    final durationMs = message.meta?['duration_ms'];
    final durationText = durationMs is num
        ? '${(durationMs / 1000).toStringAsFixed(1)}s'
        : '';
    final imageBase64 = (message.meta?['image_base64'] ?? '').toString().trim();
    Uint8List? imageBytes;
    if (isImage && imageBase64.isNotEmpty) {
      try {
        imageBytes = base64Decode(imageBase64);
      } catch (_) {
        imageBytes = null;
      }
    }

    return Align(
      alignment: alignment,
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: bubbleMaxWidth),
        child: InkWell(
          onTap: (isAudio || isApk) ? () => onTap() : null,
          onLongPress: () => onCopy(),
          borderRadius: BorderRadius.circular(18),
          child: Container(
            margin: const EdgeInsets.only(bottom: 12),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            decoration: BoxDecoration(
              color: bgColor,
              borderRadius: BorderRadius.circular(18),
              border: Border.all(
                color: isOutgoing ? palette.accent : palette.border,
              ),
              boxShadow: [
                BoxShadow(
                  blurRadius: 18,
                  color: Colors.black.withValues(alpha: 0.22),
                  offset: const Offset(0, 8),
                ),
              ],
            ),
            child: Column(
              crossAxisAlignment: isSystem
                  ? CrossAxisAlignment.center
                  : CrossAxisAlignment.start,
              children: [
                if (showAuthor) ...[
                  Text(
                    authorLabel,
                    style: TextStyle(
                      color: isOutgoing
                          ? fgColor.withValues(alpha: 0.78)
                          : palette.textMuted,
                      fontSize: 11,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 6),
                ],
                if (isImage)
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (imageBytes != null)
                        ClipRRect(
                          borderRadius: BorderRadius.circular(14),
                          child: Image.memory(
                            imageBytes,
                            fit: BoxFit.cover,
                            gaplessPlayback: true,
                          ),
                        )
                      else
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 10,
                          ),
                          decoration: BoxDecoration(
                            color: isOutgoing
                                ? accentForeground.withValues(alpha: 0.12)
                                : palette.surfaceSoft,
                            borderRadius: BorderRadius.circular(12),
                          ),
                          child: Text(
                            'Image unavailable',
                            style: TextStyle(
                              color: isSystem ? palette.textSecondary : fgColor,
                            ),
                          ),
                        ),
                      if (message.content.trim().isNotEmpty) ...[
                        const SizedBox(height: 8),
                        Text(
                          message.content,
                          style: TextStyle(
                            color: isSystem ? palette.textSecondary : fgColor,
                            height: 1.35,
                          ),
                        ),
                      ],
                    ],
                  ),
                if (isAudio)
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        isPlaying
                            ? Icons.pause_circle_filled
                            : Icons.play_circle_fill,
                        color: isSystem ? palette.textSecondary : fgColor,
                        size: 22,
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          durationText.isEmpty
                              ? '${message.content}  Tap to play'
                              : '${message.content}  $durationText  Tap to play',
                          style: TextStyle(
                            color: isSystem ? palette.textSecondary : fgColor,
                            height: 1.35,
                          ),
                        ),
                      ),
                    ],
                  ),
                if (!isAudio && !isImage)
                  Text(
                    isApk
                        ? '${message.content}${extractApkVersion(message) != null ? '\n版本: ${extractApkVersion(message)}' : ''}\n点击安装 APK'
                        : message.content,
                    style: TextStyle(
                      color: isSystem ? palette.textSecondary : fgColor,
                      height: 1.35,
                    ),
                  ),
                const SizedBox(height: 6),
                Text(
                  isImage
                      ? '${_formatTime(message.timestamp)}  Long press to copy'
                      : isAudio
                      ? '${_formatTime(message.timestamp)}  Tap to play · Long press to copy'
                      : isApk
                      ? '${_formatTime(message.timestamp)}  ${extractApkVersion(message) != null ? 'v${extractApkVersion(message)} · ' : ''}点击安装 · 长按复制'
                      : '${_formatTime(message.timestamp)}  Long press to copy',
                  style: TextStyle(
                    fontSize: 11,
                    color: isOutgoing
                        ? fgColor.withValues(alpha: 0.74)
                        : palette.textMuted,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  static String _formatTime(DateTime time) {
    final hh = time.hour.toString().padLeft(2, '0');
    final mm = time.minute.toString().padLeft(2, '0');
    final ss = time.second.toString().padLeft(2, '0');
    return '$hh:$mm:$ss';
  }
}

class _VoiceWaveformBadge extends StatefulWidget {
  const _VoiceWaveformBadge();

  @override
  State<_VoiceWaveformBadge> createState() => _VoiceWaveformBadgeState();
}

class _VoiceWaveformBadgeState extends State<_VoiceWaveformBadge>
    with SingleTickerProviderStateMixin {
  static const List<double> _baseHeights = <double>[10, 16, 24, 34, 24, 16, 10];
  static const List<double> _amplitudes = <double>[7, 10, 14, 18, 14, 10, 7];

  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1100),
  )..repeat();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final palette = context.appPalette;
    final waveformColor = _foregroundForColor(palette.accent);
    return SizedBox(
      height: 54,
      child: AnimatedBuilder(
        animation: _controller,
        builder: (context, _) {
          final phase = _controller.value * math.pi * 2;
          final innerPulse = 0.94 + 0.14 * (0.5 + 0.5 * math.sin(phase));
          final outerPulse = 1.02 + 0.22 * (0.5 + 0.5 * math.sin(phase - 1.1));

          return Stack(
            alignment: Alignment.center,
            children: [
              Transform.scale(
                scale: outerPulse,
                child: Container(
                  width: 56,
                  height: 56,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: waveformColor.withValues(alpha: 0.08),
                  ),
                ),
              ),
              Transform.scale(
                scale: innerPulse,
                child: Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: waveformColor.withValues(alpha: 0.12),
                  ),
                ),
              ),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  for (var i = 0; i < _baseHeights.length; i++) ...[
                    Builder(
                      builder: (context) {
                        final wave =
                            0.5 + 0.5 * math.sin(phase * 1.9 + i * 0.82);
                        final height =
                            _baseHeights[i] + _amplitudes[i] * wave;
                        final alpha = (0.46 + wave * 0.46).clamp(0.0, 1.0);
                        return Container(
                          width: 4,
                          height: height,
                          decoration: BoxDecoration(
                            color: waveformColor.withValues(alpha: alpha),
                            borderRadius: BorderRadius.circular(999),
                            boxShadow: [
                              BoxShadow(
                                blurRadius: 10,
                                color: waveformColor.withValues(
                                  alpha: alpha * 0.22,
                                ),
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                    if (i != _baseHeights.length - 1) const SizedBox(width: 6),
                  ],
                ],
              ),
            ],
          );
        },
      ),
    );
  }
}

class _VoiceActionPill extends StatelessWidget {
  const _VoiceActionPill({
    required this.label,
    required this.active,
    required this.alignment,
  });

  final String label;
  final bool active;
  final Alignment alignment;

  @override
  Widget build(BuildContext context) {
    final palette = context.appPalette;
    return AnimatedContainer(
      duration: const Duration(milliseconds: 160),
      width: 150,
      height: 58,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      decoration: BoxDecoration(
        color: active
            ? palette.accent
            : palette.surfaceRaised.withValues(alpha: 0.92),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(
          color: active
              ? _foregroundForColor(palette.accent).withValues(alpha: 0.72)
              : palette.borderStrong.withValues(alpha: 0.78),
        ),
      ),
      child: Align(
        alignment: alignment,
        child: Text(
          label,
          style: TextStyle(
            color: active
                ? _foregroundForColor(palette.accent)
                : palette.textSecondary,
            fontSize: 18,
            fontWeight: FontWeight.w700,
            letterSpacing: 0.2,
          ),
        ),
      ),
    );
  }
}
