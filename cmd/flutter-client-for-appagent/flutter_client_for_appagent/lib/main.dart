import 'dart:async';
import 'dart:convert'
    show
        JsonEncoder,
        base64Decode,
        base64Encode,
        jsonDecode,
        jsonEncode,
        utf8;
import 'dart:io';
import 'dart:math' as math;

import 'package:archive/archive.dart';
import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart' hide AndroidOptions;
import 'package:http/http.dart' as http;
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:speech_to_text/speech_to_text.dart' as stt;
import 'package:web_socket_channel/web_socket_channel.dart';

import 'codegen/codegen_body.dart';
import 'codegen/models.dart';
import 'cortana_broadcast_queue.dart';
import 'cortana_history_page.dart';
import 'cortana_page.dart'
    show
        CortanaDisplayMode,
        CortanaPage,
        CortanaPageState,
        CortanaReplayItem,
        CortanaReplyPayload,
        CortanaSettings,
        FlutterClientLogEntry,
        addFlutterClientLog,
        flutterClientLogs;
import 'speech_transcript_formatter.dart';
import 'version.g.dart';
import 'vosk_model_locator.dart';

void main() {
  runApp(const AppAgentClientApp());
}

bool get _isAndroidHost => !kIsWeb && Platform.isAndroid;
bool get _isWindowsHost => !kIsWeb && Platform.isWindows;

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
            _toneFromAccent(
              accentHsl,
              lightness: 0.975,
              saturationFactor: 0.14,
            ),
            accent,
            0.03,
          );
    final backgroundBottom = isDark
        ? _blendWithAccent(
            _toneFromAccent(
              accentHsl,
              lightness: 0.055,
              saturationFactor: 0.28,
            ),
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
            _toneFromAccent(
              accentHsl,
              lightness: 0.945,
              saturationFactor: 0.10,
            ),
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
        ? _toneFromAccent(accentHsl, lightness: 0.965, saturationFactor: 0.06)
        : _toneFromAccent(accentHsl, lightness: 0.10, saturationFactor: 0.10);
    final textSecondary = isDark
        ? _toneFromAccent(accentHsl, lightness: 0.78, saturationFactor: 0.12)
        : _toneFromAccent(accentHsl, lightness: 0.30, saturationFactor: 0.10);
    final textMuted = isDark
        ? _toneFromAccent(accentHsl, lightness: 0.62, saturationFactor: 0.16)
        : _toneFromAccent(accentHsl, lightness: 0.48, saturationFactor: 0.10);
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
      backgroundBottom: Color.lerp(
        backgroundBottom,
        other.backgroundBottom,
        t,
      )!,
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
  final colorScheme =
      (preset.brightness == Brightness.dark
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

class LlmDebugEvent {
  const LlmDebugEvent({
    required this.event,
    required this.content,
    required this.timestamp,
    required this.meta,
  });

  final String event;
  final String content;
  final DateTime timestamp;
  final Map<String, dynamic> meta;

  String get label {
    switch (event) {
      case 'debug_prompt':
        return '提示词';
      case 'debug_llm_round':
        return 'LLM';
      case 'tool_call':
        return '工具调用';
      case 'tool_result':
        return '工具结果';
      case 'thinking':
        return '思考';
      case 'tool_info':
        return '工具';
      case 'task_complete':
        return '完成';
      default:
        return event;
    }
  }

  Map<String, dynamic> get payload {
    final raw = meta['debug_payload'];
    if (raw is Map<String, dynamic>) {
      return raw;
    }
    if (raw is Map) {
      return Map<String, dynamic>.from(raw);
    }
    return const <String, dynamic>{};
  }

  String get detailText {
    final data = payload;
    if (event == 'debug_prompt') {
      final prompt = (data['system_prompt'] ?? '').toString();
      if (prompt.trim().isNotEmpty) {
        return prompt;
      }
    }
    if (event == 'debug_llm_round') {
      final assistant = (data['assistant_text'] ?? '').toString();
      final calls = data['tool_calls'];
      final parts = <String>[];
      if (assistant.trim().isNotEmpty) {
        parts.add(assistant);
      }
      if (calls is List && calls.isNotEmpty) {
        parts.add(const JsonEncoder.withIndent('  ').convert(calls));
      }
      if (parts.isNotEmpty) {
        return parts.join('\n\n');
      }
    }
    return content;
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
    sanitizedMeta?.remove('video_base64');
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

typedef ScopedHistoryPersistCallback = Future<void> Function(String scopeKey);

String resolvePreferredGroupId(
  List<GroupInfo> groups, {
  String? preferredGroupId,
  bool allowImplicitSingleSelection = true,
}) {
  if (groups.isEmpty) {
    return '';
  }
  final preferred = preferredGroupId?.trim() ?? '';
  if (preferred.isNotEmpty && groups.any((group) => group.id == preferred)) {
    return preferred;
  }
  if (allowImplicitSingleSelection && groups.length == 1) {
    return groups.first.id;
  }
  return '';
}

class ScopedHistoryPersistenceCoordinator {
  ScopedHistoryPersistenceCoordinator(this._persist);

  final ScopedHistoryPersistCallback _persist;
  final Map<String, Future<void>> _tails = <String, Future<void>>{};
  final Map<String, int> _revisions = <String, int>{};
  int _epoch = 0;

  void schedule(String scopeKey) {
    final normalizedScopeKey = scopeKey.trim();
    if (normalizedScopeKey.isEmpty) {
      return;
    }
    final scheduledEpoch = _epoch;
    final revision = (_revisions[normalizedScopeKey] ?? 0) + 1;
    _revisions[normalizedScopeKey] = revision;

    final previous = _tails[normalizedScopeKey] ?? Future<void>.value();
    late final Future<void> future;
    future = previous.catchError((_) {}).then((_) async {
      // 同一 scope 的并发写只落最后一次，避免旧快照覆盖新消息。
      if (_epoch != scheduledEpoch) {
        return;
      }
      if (_revisions[normalizedScopeKey] != revision) {
        return;
      }
      await _persist(normalizedScopeKey);
    });
    _tails[normalizedScopeKey] = future.whenComplete(() {
      if (identical(_tails[normalizedScopeKey], future)) {
        _tails.remove(normalizedScopeKey);
      }
    });
  }

  void invalidate() {
    _epoch++;
    _revisions.clear();
  }

  Future<void> flushScope(String scopeKey) async {
    final normalizedScopeKey = scopeKey.trim();
    if (normalizedScopeKey.isEmpty) {
      return;
    }
    schedule(normalizedScopeKey);
    final tail = _tails[normalizedScopeKey];
    if (tail == null) {
      return;
    }
    await tail.catchError((_) {});
  }

  Future<void> flushAll([Iterable<String>? scopeKeys]) async {
    final normalizedScopeKeys =
        (scopeKeys ?? <String>[..._revisions.keys, ..._tails.keys])
            .map((scopeKey) => scopeKey.trim())
            .where((scopeKey) => scopeKey.isNotEmpty)
            .toSet()
            .toList();
    if (normalizedScopeKeys.isEmpty) {
      return;
    }
    for (final scopeKey in normalizedScopeKeys) {
      schedule(scopeKey);
    }
    await Future.wait(normalizedScopeKeys.map(flushScope), eagerError: false);
  }
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

enum RootTab { chat, codegen, cortana, debug, settings }

class ClientConfig {
  const ClientConfig({
    required this.baseUrl,
    required this.receiveToken,
    required this.enableLocalVosk,
    required this.voskModelPath,
    required this.cortanaEnabledDefault,
    required this.cortanaAllowFullAccessDefault,
    required this.cortanaAutoPlayDefault,
    required this.cortanaProactiveModeDefault,
    required this.cortanaHighFreqStartHourDefault,
    required this.cortanaHighFreqStartMinuteDefault,
    required this.cortanaHighFreqEndHourDefault,
    required this.cortanaHighFreqEndMinuteDefault,
    required this.cortanaPersonaNameDefault,
    required this.cortanaPersonaDescriptionDefault,
  });

  final String baseUrl;
  final String receiveToken;
  final bool enableLocalVosk;
  final String voskModelPath;
  final bool cortanaEnabledDefault;
  final bool cortanaAllowFullAccessDefault;
  final bool cortanaAutoPlayDefault;
  final String cortanaProactiveModeDefault;
  final int cortanaHighFreqStartHourDefault;
  final int cortanaHighFreqStartMinuteDefault;
  final int cortanaHighFreqEndHourDefault;
  final int cortanaHighFreqEndMinuteDefault;
  final String cortanaPersonaNameDefault;
  final String cortanaPersonaDescriptionDefault;

  factory ClientConfig.fromJson(Map<String, dynamic> json) {
    return ClientConfig(
      baseUrl: (json['base_url'] ?? '').toString().trim(),
      receiveToken: (json['receive_token'] ?? '').toString().trim(),
      enableLocalVosk: json['enable_local_vosk'] == true,
      voskModelPath: (json['vosk_model_path'] ?? '').toString().trim(),
      cortanaEnabledDefault: json['cortana_enabled_default'] != false,
      cortanaAllowFullAccessDefault:
          json['cortana_allow_full_access_default'] != false,
      cortanaAutoPlayDefault: json['cortana_auto_play_default'] != false,
      cortanaProactiveModeDefault:
          (json['cortana_proactive_mode_default'] ?? 'high').toString().trim(),
      cortanaHighFreqStartHourDefault:
          (json['cortana_high_freq_start_hour_default'] as num?)?.toInt() ?? 9,
      cortanaHighFreqStartMinuteDefault:
          (json['cortana_high_freq_start_minute_default'] as num?)?.toInt() ??
          0,
      cortanaHighFreqEndHourDefault:
          (json['cortana_high_freq_end_hour_default'] as num?)?.toInt() ?? 22,
      cortanaHighFreqEndMinuteDefault:
          (json['cortana_high_freq_end_minute_default'] as num?)?.toInt() ?? 0,
      cortanaPersonaNameDefault:
          (json['cortana_persona_name_default'] ?? 'Cortana').toString().trim(),
      cortanaPersonaDescriptionDefault:
          (json['cortana_persona_description_default'] ?? '').toString().trim(),
    );
  }
}

class AppAgentUnauthorizedException implements Exception {
  const AppAgentUnauthorizedException(this.message);

  final String message;

  @override
  String toString() => message;
}

class AppAuthSession {
  const AppAuthSession({
    required this.userId,
    required this.accessToken,
    required this.refreshToken,
    required this.expiresAtMs,
    required this.obsAgentBaseUrl,
  });

  final String userId;
  final String accessToken;
  final String refreshToken;
  final int expiresAtMs;
  final String obsAgentBaseUrl;

  factory AppAuthSession.fromJson(
    Map<String, dynamic> json, {
    String fallbackUserId = '',
    String fallbackRefreshToken = '',
    String fallbackObsAgentBaseUrl = '',
  }) {
    final accessToken = (json['access_token'] ?? json['session_token'] ?? '')
        .toString()
        .trim();
    final refreshToken = (json['refresh_token'] ?? fallbackRefreshToken)
        .toString()
        .trim();
    final userId = (json['user_id'] ?? fallbackUserId).toString().trim();
    final obsAgentBaseUrl =
        (json['obs_agent_base_url'] ?? fallbackObsAgentBaseUrl)
            .toString()
            .trim();
    final expiresAtMs = _resolveExpiresAtMs(json);
    if (accessToken.isEmpty) {
      throw const FormatException('missing access_token');
    }
    if (userId.isEmpty) {
      throw const FormatException('missing user_id');
    }
    return AppAuthSession(
      userId: userId,
      accessToken: accessToken,
      refreshToken: refreshToken,
      expiresAtMs: expiresAtMs,
      obsAgentBaseUrl: obsAgentBaseUrl,
    );
  }

  static int _resolveExpiresAtMs(Map<String, dynamic> json) {
    final rawExpiresAt = json['expires_at'];
    if (rawExpiresAt is int) {
      return rawExpiresAt;
    }
    final parsedExpiresAt = int.tryParse('${json['expires_at']}') ?? 0;
    if (parsedExpiresAt > 0) {
      return parsedExpiresAt;
    }
    final rawExpiresIn = json['expires_in'];
    final expiresIn = rawExpiresIn is int
        ? rawExpiresIn
        : int.tryParse('${json['expires_in']}') ?? 0;
    if (expiresIn <= 0) {
      return 0;
    }
    return DateTime.now().millisecondsSinceEpoch + expiresIn * 1000;
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

  Map<String, String> _sessionHeaders() {
    return <String, String>{
      if (receiveToken.trim().isNotEmpty)
        'X-App-Agent-Token': receiveToken.trim(),
      if (sessionToken.trim().isNotEmpty)
        'X-App-Agent-Session': sessionToken.trim(),
    };
  }

  Never _throwRequestError(String operation, http.Response resp) {
    final body = resp.body.trim();
    if (resp.statusCode == HttpStatus.unauthorized) {
      throw AppAgentUnauthorizedException(
        '$operation failed: ${resp.statusCode} $body',
      );
    }
    throw HttpException('$operation failed: ${resp.statusCode} $body');
  }

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

  Future<AppAuthSession> login() async {
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
      _throwRequestError('login', resp);
    }
    return AppAuthSession.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
      fallbackUserId: userId,
    );
  }

  Future<AppAuthSession> refreshSession(String refreshToken) async {
    final uri = Uri.parse('$baseUrl/api/app/refresh');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json',
            if (receiveToken.trim().isNotEmpty)
              'X-App-Agent-Token': receiveToken.trim(),
          },
          body: jsonEncode({
            'user_id': userId,
            'refresh_token': refreshToken.trim(),
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('refresh', resp);
    }
    return AppAuthSession.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
      fallbackUserId: userId,
      fallbackRefreshToken: refreshToken,
    );
  }

  Future<void> logout({String refreshToken = ''}) async {
    final uri = Uri.parse('$baseUrl/api/app/logout');
    final resp = await http
        .post(
          uri,
          headers: <String, String>{
            HttpHeaders.contentTypeHeader: 'application/json',
            ..._sessionHeaders(),
          },
          body: jsonEncode({
            'user_id': userId,
            if (refreshToken.trim().isNotEmpty)
              'refresh_token': refreshToken.trim(),
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('logout', resp);
    }
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
            ..._sessionHeaders(),
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
      _throwRequestError('send', resp);
    }
  }

  Future<void> sendMessage(String content) => sendAppMessage(content);

  Future<void> sendCortanaEvent(
    String eventKind, {
    String content = '',
    Map<String, dynamic>? meta,
  }) {
    return sendAppMessage(
      content,
      messageType: 'event',
      meta: <String, dynamic>{
        'event_kind': eventKind,
        if (meta != null) ...meta,
      },
    );
  }

  Future<Map<String, dynamic>> fetchCortanaSettings() async {
    final uri = Uri.parse(
      '$baseUrl/api/app/cortana/settings?user_id=$userId&session_token=$sessionToken',
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('get cortana settings', resp);
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> saveCortanaSettings(
    Map<String, dynamic> settings,
  ) async {
    final uri = Uri.parse('$baseUrl/api/app/cortana/settings');
    final resp = await http
        .post(
          uri,
          headers: {
            HttpHeaders.contentTypeHeader: 'application/json',
            ..._sessionHeaders(),
          },
          body: jsonEncode(<String, dynamic>{'user_id': userId, ...settings}),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('save cortana settings', resp);
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  Future<List<GroupInfo>> listGroups() async {
    final uri = Uri.parse(
      '$baseUrl/api/app/groups?user_id=$userId&session_token=$sessionToken',
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list groups', resp);
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
            ..._sessionHeaders(),
          },
          body: jsonEncode({
            'action': action,
            'user_id': userId,
            'group_id': groupId,
          }),
        )
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('group $action', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final groups = (data['groups'] as List<dynamic>? ?? const [])
        .map((item) => GroupInfo.fromJson(item as Map<String, dynamic>))
        .toList();
    return groups;
  }

  Future<CodegenProjectsSnapshot> listCodegenProjects() async {
    final uri = Uri.parse(
      '$baseUrl/api/app/codegen/projects?user_id=$userId&session_token=$sessionToken',
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list codegen projects', resp);
    }
    return CodegenProjectsSnapshot.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
    );
  }

  Future<List<CortanaReplayItem>> listCortanaVoiceHistory({
    int limit = 200,
  }) async {
    final uri = Uri.parse(
      '$baseUrl/api/app/cortana/history?user_id=$userId&session_token=$sessionToken&limit=$limit',
    );
    final resp = await http
        .get(uri, headers: _sessionHeaders())
        .timeout(_httpTimeout);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwRequestError('list cortana history', resp);
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    final items = (data['items'] as List<dynamic>? ?? const [])
        .map((item) {
          final json = item as Map<String, dynamic>;
          final createdAtValue = json['created_at'];
          final createdAtMillis = createdAtValue is int
              ? createdAtValue
              : int.tryParse('$createdAtValue') ??
                    DateTime.now().millisecondsSinceEpoch;
          return CortanaReplayItem(
            id: (json['id'] ?? '').toString().trim(),
            text: (json['text'] ?? '').toString().trim(),
            audioPath: '',
            audioFormat: (json['audio_format'] ?? '').toString().trim(),
            createdAt: DateTime.fromMillisecondsSinceEpoch(createdAtMillis),
            sourceLabel: '服务端历史',
            fileId: (json['file_id'] ?? '').toString().trim(),
            storageProvider: (json['storage_provider'] ?? '').toString().trim(),
            objectKey: (json['object_key'] ?? '').toString().trim(),
          );
        })
        .where((item) => item.text.isNotEmpty && item.fileId.isNotEmpty)
        .toList();
    return items;
  }

  Future<WebSocketChannel> connectWebSocket() async {
    final uri = _buildWsUri(baseUrl, userId, sessionToken, receiveToken);
    final channel = WebSocketChannel.connect(uri);
    await channel.ready.timeout(_wsConnectTimeout);
    return channel;
  }

  Future<List<int>> downloadAttachment(String fileId) async {
    final uri = _buildAttachmentUri(fileId);
    final resp = await http.get(uri, headers: _attachmentHeaders());
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      if (resp.statusCode == HttpStatus.unauthorized) {
        throw AppAgentUnauthorizedException(
          'download attachment failed: ${resp.statusCode} ${resp.body}',
        );
      }
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
    final uri = _buildAttachmentUri(fileId);
    await downloader.downloadToFile(
      uri,
      destinationPath: destinationPath,
      headersBuilder: ({int? rangeStart}) =>
          _attachmentHeaders(rangeStart: rangeStart),
      onProgress: onProgress,
    );
  }

  static Uri _buildWsUri(
    String baseUrl,
    String userId,
    String sessionToken,
    String receiveToken,
  ) {
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
        if (receiveToken.trim().isNotEmpty) 'token': receiveToken.trim(),
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

class _CodegenStreamState {
  _CodegenStreamState({
    required this.scopeKey,
    required this.streamMessageId,
    required this.latestMessage,
  });

  String scopeKey;
  final String streamMessageId;
  ChatMessage latestMessage;
  String fullContent = '';
  String pendingDelta = '';
  int segmentIndex = 0;
  String? activeSegmentMessageId;
  bool finalSeen = false;
}

const bool _defaultCortanaEnabled = true;
const bool _defaultCortanaAllowFullAccess = true;
const bool _defaultCortanaAutoPlay = true;
const String _defaultCortanaProactiveMode = 'high';
const bool _defaultCortanaVoiceWakeEnabled = false;
const String _defaultCortanaWakePhrase = '嗨 Cortana';
const String _defaultCortanaOwnerTitle = '';

class _ChatPageState extends State<ChatPage> with WidgetsBindingObserver {
  static const String _baseUrlOverrideKey = 'client_config::base_url_override';
  static const String _lastLoginUserIdKey = 'auth::last_user_id';
  static const String _refreshTokenStorageKey = 'auth::refresh_token';
  static const String _historyStoragePrefix = 'chat_history::';
  static const String _historyBackupStoragePrefix = 'chat_history_secure::';
  static const String _lastReadAtStoragePrefix = 'chat_last_read_at';
  static const String _codegenModeKey = 'codegen::last_mode';
  static const String _codeProjectKey = 'codegen::last_code_project';
  static const String _codeToolKey = 'codegen::last_code_tool';
  static const String _claudeSettingsKey = 'codegen::last_claude_settings';
  static const String _deployProjectKey = 'codegen::last_deploy_project';
  static const String _deployTargetKey = 'codegen::last_deploy_target';
  static const String _deployArgsKey = 'codegen::last_deploy_args';
  static const String _codegenHistoryKey = 'codegen::history';
  static const String _cortanaEnabledKey = 'cortana::enabled';
  static const String _cortanaAllowFullAccessKey = 'cortana::allow_full_access';
  static const String _cortanaAutoPlayKey = 'cortana::auto_play';
  static const String _cortanaProactiveModeKey = 'cortana::proactive_mode';
  static const String _cortanaHighFreqStartHourKey =
      'cortana::high_freq_start_hour';
  static const String _cortanaHighFreqStartMinuteKey =
      'cortana::high_freq_start_minute';
  static const String _cortanaHighFreqEndHourKey =
      'cortana::high_freq_end_hour';
  static const String _cortanaHighFreqEndMinuteKey =
      'cortana::high_freq_end_minute';
  static const String _cortanaPersonaNameKey = 'cortana::persona_name';
  static const String _cortanaOwnerTitleKey = 'cortana::owner_title';
  static const String _cortanaPersonaDescriptionKey =
      'cortana::persona_description';
  static const String _cortanaVoiceWakeEnabledKey =
      'cortana::voice_wake_enabled';
  static const String _cortanaWakePhraseKey = 'cortana::wake_phrase';
  static const Duration _sessionRefreshSkew = Duration(minutes: 1);
  static final FlutterSecureStorage _secureStorage = const FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
    iOptions: IOSOptions(
      accessibility: KeychainAccessibility.first_unlock_this_device,
    ),
  );
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
  final _codegenPromptController = TextEditingController();
  final _codegenSearchController = TextEditingController();
  final _deployArgsController = TextEditingController();
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
  final DeviceLocationProvider _locationProvider = DeviceLocationProvider();
  final ResumableFileDownloader _fileDownloader = const ResumableFileDownloader(
    retryDelays: _voskDownloadRetryDelays,
  );

  final Map<String, List<ChatMessage>> _historyByScope =
      <String, List<ChatMessage>>{};
  final Map<String, GlobalKey> _messageAnchorKeys = <String, GlobalKey>{};
  late final ScopedHistoryPersistenceCoordinator _historyPersistence =
      ScopedHistoryPersistenceCoordinator(_persistHistory);
  final List<GroupInfo> _groups = <GroupInfo>[];
  final List<CodingProjectInfo> _codingProjects = <CodingProjectInfo>[];
  final List<DeployProjectInfo> _deployProjects = <DeployProjectInfo>[];
  final List<LlmDebugEvent> _llmDebugEvents = <LlmDebugEvent>[];
  final Set<String> _seenMessageIds = <String>{};
  final Set<String> _autoInstallTriggered = <String>{};
  final Set<String> _consumedCortanaReplyKeys = <String>{};

  WebSocketChannel? _socket;
  StreamSubscription<dynamic>? _socketSub;
  Timer? _reconnectTimer;
  Timer? _cortanaWakeRestartTimer;
  Timer? _cortanaLocationTimer;
  Timer? _streamFlushTimer;
  final Map<String, _CodegenStreamState> _codegenStreamStates =
      <String, _CodegenStreamState>{};
  final Set<String> _pendingCodegenStreamIds = <String>{};
  static const Duration _streamFlushInterval = Duration(milliseconds: 80);
  static const int _codegenStreamSegmentLimit = 2200;
  bool _scrollToBottomScheduled = false;

  bool _connecting = false;
  bool _connected = false;
  bool _loggingIn = false;
  bool _recording = false;
  bool _speechReady = false;
  bool _systemSpeechReady = false;
  bool _useLocalVosk = false;
  bool _sending = false;
  bool _transcribingVoice = false;
  bool _voiceInputMode = false;
  String? _playingAudioKey;
  bool _autoReconnect = false;
  bool _configLoading = true;
  bool _sidebarExpanded = false;
  bool _controlsExpanded = false;
  bool _groupTabsExpanded = false;
  bool _passwordVisible = false;
  bool _codegenLoading = false;
  bool _codegenSending = false;
  bool _cortanaEnabled = _defaultCortanaEnabled;
  bool _cortanaAllowFullAccess = _defaultCortanaAllowFullAccess;
  bool _cortanaAutoPlay = _defaultCortanaAutoPlay;
  String _cortanaProactiveMode = _defaultCortanaProactiveMode;
  int _cortanaHighFreqStartHour = 9;
  int _cortanaHighFreqStartMinute = 0;
  int _cortanaHighFreqEndHour = 22;
  int _cortanaHighFreqEndMinute = 0;
  String _cortanaPersonaName = 'Cortana';
  String _cortanaOwnerTitle = _defaultCortanaOwnerTitle;
  String _cortanaPersonaDescription = '';
  bool _cortanaVoiceWakeEnabled = _defaultCortanaVoiceWakeEnabled;
  String _cortanaWakePhrase = _defaultCortanaWakePhrase;
  final TextEditingController _cortanaPersonaNameCtrl = TextEditingController();
  final TextEditingController _cortanaOwnerTitleCtrl = TextEditingController();
  final TextEditingController _cortanaPersonaDescCtrl = TextEditingController();
  final TextEditingController _cortanaWakePhraseCtrl = TextEditingController();
  bool _cortanaChatSettingsExpanded = false;
  bool _cortanaChatLogsExpanded = false;
  bool _codegenAutoDeploy = false;
  bool _deployPackOnly = false;
  List<CodegenHistoryItem> _codegenHistory = [];
  String _activeCodegenHistoryId = '';
  int _lastSequence = 0;
  String _status = 'Idle';
  String _sessionToken = '';
  String _refreshToken = '';
  int _sessionExpiresAtMs = 0;
  String _obsAgentBaseUrl = '';
  String _currentGroupId = '';
  String _configError = '';
  Offset _recordDragOffset = Offset.zero;
  Offset? _recordDragStartGlobalPosition;
  String _speechDraft = '';
  String _codegenError = '';
  String _selectedCodeProjectQualifiedName = '';
  String _selectedCodeTool = '';
  String _selectedClaudeSettings = '';
  String _selectedDeployProjectQualifiedName = '';
  String _selectedDeployTarget = '';
  DateTime? _recordStartedAt;
  ClientConfig? _clientConfig;
  String? _downloadStatusLabel;
  int _downloadStatusPercent = -1;
  bool _voskModelDownloading = false;
  double _voskModelDownloadProgress = 0.0;
  String? _voskModelDownloadError;
  Future<bool>? _sessionRefreshFuture;
  RootTab _rootTab = RootTab.chat;
  CortanaDisplayMode _cortanaFloatingMode = CortanaDisplayMode.collapsed;
  final GlobalKey<CortanaPageState> _cortanaPageKey =
      GlobalKey<CortanaPageState>();
  bool _cortanaBadge = false;
  final CortanaBroadcastQueue _cortanaBroadcastQueue = CortanaBroadcastQueue();
  String? _cortanaContextualExpression;
  CodegenLaunchMode _codegenMode = CodegenLaunchMode.code;
  bool _startupGreetingShown = false;
  bool _loginGreetingShown = false;
  bool _cortanaWakeListening = false;
  bool _cortanaWakeHandling = false;
  bool _cortanaWakePausedForLifecycle = false;
  bool _speechTransitioning = false;
  Future<void> _speechStopTail = Future<void>.value();
  String _lastSpeechStatusLog = '';
  String _lastCortanaWakeTranscript = '';
  bool _cortanaLocationUpdating = false;
  Map<String, dynamic>? _lastCortanaDeviceContext;
  DateTime? _lastCortanaLocationReportAt;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _appendSystem('Loading client config...');
    unawaited(_restoreCodegenPreferences());
    unawaited(_loadCodegenHistory());
    unawaited(_loadClientConfig());
    unawaited(_restoreVoskDownloadProgress());
    _scheduleCortanaLocationRefresh(initial: true);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.inactive ||
        state == AppLifecycleState.hidden ||
        state == AppLifecycleState.paused ||
        state == AppLifecycleState.detached) {
      _cortanaWakePausedForLifecycle = true;
      unawaited(_pauseCortanaWakeListening(cancel: true));
      unawaited(_flushHistoryToDisk());
    } else if (state == AppLifecycleState.resumed) {
      _cortanaWakePausedForLifecycle = false;
      _scheduleCortanaWakeRestart();
      _scheduleCortanaLocationRefresh(initial: false);
    }
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
    WidgetsBinding.instance.removeObserver(this);
    unawaited(_flushHistoryToDisk());
    _reconnectTimer?.cancel();
    _cortanaExpressionTimer?.cancel();
    _cortanaWakeRestartTimer?.cancel();
    _cortanaLocationTimer?.cancel();
    _streamFlushTimer?.cancel();
    unawaited(_socketSub?.cancel());
    unawaited(_socket?.sink.close());
    unawaited(_pauseCortanaWakeListening(cancel: true));
    _userIdController.dispose();
    _passwordController.dispose();
    _baseUrlController.dispose();
    _groupIdController.dispose();
    _messageController.dispose();
    _codegenPromptController.dispose();
    _codegenSearchController.dispose();
    _deployArgsController.dispose();
    _cortanaPersonaNameCtrl.dispose();
    _cortanaOwnerTitleCtrl.dispose();
    _cortanaPersonaDescCtrl.dispose();
    _cortanaWakePhraseCtrl.dispose();
    _messageFocusNode.dispose();
    _scrollController.dispose();
    _controlsScrollController.dispose();
    unawaited(_audioPlayer.dispose());
    unawaited(_audioRecorder.dispose());
    super.dispose();
  }

  Future<void> _flushHistoryToDisk() async {
    try {
      await _historyPersistence.flushAll(_historyByScope.keys);
    } catch (err) {
      debugPrint('Flush history failed: $err');
    }
  }

  bool _isSpeechDoneStatus(String status) {
    final normalized = status.toLowerCase().trim();
    return normalized == 'done' ||
        normalized == 'notlistening' ||
        normalized == 'not_listening';
  }

  void _handleSpeechRecognitionStatus(String status) {
    if (_lastSpeechStatusLog != status) {
      _lastSpeechStatusLog = status;
      _appendSystem('Speech recognition status: $status');
    }
    if (_isSpeechDoneStatus(status) && _cortanaWakeListening) {
      _cortanaWakeListening = false;
      unawaited(_handleCortanaWakeSessionEnded());
    }
  }

  void _handleSpeechRecognitionError(Object error) {
    _appendSystem('Speech recognition error: $error');
    if (_isSpeechBusyError(error)) {
      unawaited(_resetBusySpeechRecognition());
      return;
    }
    if (_cortanaWakeListening) {
      _cortanaWakeListening = false;
      if (!_cortanaWakeHandling) {
        _scheduleCortanaWakeRestart(delay: const Duration(seconds: 2));
      }
    }
  }

  bool _isSpeechBusyError(Object error) {
    final text = error.toString().toLowerCase();
    return text.contains('error_busy') || text.contains('recognizer_busy');
  }

  Future<void> _resetBusySpeechRecognition() async {
    _cortanaWakeListening = false;
    await _stopSpeechRecognition(cancel: true);
    if (!_cortanaWakeHandling) {
      _scheduleCortanaWakeRestart(delay: const Duration(seconds: 2));
    }
  }

  Future<void> _stopSpeechRecognition({required bool cancel}) {
    final previous = _speechStopTail;
    late final Future<void> next;
    next = previous.catchError((_) {}).then((_) async {
      _speechTransitioning = true;
      try {
        if (cancel) {
          await _speechToText.cancel();
        } else if (_speechToText.isListening) {
          await _speechToText.stop();
        }
      } catch (_) {
        try {
          await _speechToText.cancel();
        } catch (_) {}
      }
      await Future<void>.delayed(const Duration(milliseconds: 900));
      _speechTransitioning = false;
    });
    _speechStopTail = next;
    return next;
  }

  Future<void> _handleCortanaWakeSessionEnded() async {
    await _stopSpeechRecognition(cancel: true);
    if (!_cortanaWakeHandling) {
      _scheduleCortanaWakeRestart(delay: const Duration(milliseconds: 1200));
    }
  }

  Future<bool> _ensureSystemSpeechRecognitionReady({
    bool silent = false,
  }) async {
    if (_systemSpeechReady) {
      return true;
    }
    try {
      final available = await _speechToText.initialize(
        onError: _handleSpeechRecognitionError,
        onStatus: _handleSpeechRecognitionStatus,
      );
      if (!mounted) {
        return false;
      }
      if (!available && !silent) {
        _appendSystem('Speech recognition not available on this device.');
      }
      setState(() {
        _systemSpeechReady = available;
      });
      return available;
    } catch (err, stack) {
      if (!mounted) {
        return false;
      }
      if (!silent) {
        _appendSystem('Speech recognition init failed: $err');
      }
      debugPrint('Speech init error: $err\n$stack');
      setState(() {
        _systemSpeechReady = false;
      });
      return false;
    }
  }

  Future<void> _initVoice() async {
    final config = _clientConfig;
    final prefs = await SharedPreferences.getInstance();
    if (_isAndroidHost && config != null && config.enableLocalVosk) {
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
            await _ensureSystemSpeechRecognitionReady(silent: true);
            _scheduleCortanaWakeRestart();
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

    final available = await _ensureSystemSpeechRecognitionReady();
    if (!mounted) {
      return;
    }
    setState(() {
      _speechReady = available;
      _useLocalVosk = false;
    });
    _scheduleCortanaWakeRestart();
  }

  bool get _canListenForCortanaWake =>
      mounted &&
      _cortanaEnabled &&
      _cortanaVoiceWakeEnabled &&
      !_cortanaWakePausedForLifecycle &&
      !_recording &&
      !_sending &&
      !_transcribingVoice &&
      !_cortanaWakeHandling &&
      !_speechTransitioning;

  void _scheduleCortanaWakeRestart({
    Duration delay = const Duration(milliseconds: 500),
  }) {
    _cortanaWakeRestartTimer?.cancel();
    if (!_canListenForCortanaWake) {
      return;
    }
    _cortanaWakeRestartTimer = Timer(delay, () {
      if (!mounted || !_canListenForCortanaWake) {
        return;
      }
      unawaited(_startCortanaWakeListening());
    });
  }

  Future<void> _pauseCortanaWakeListening({required bool cancel}) async {
    _cortanaWakeRestartTimer?.cancel();
    _cortanaWakeListening = false;
    await _stopSpeechRecognition(cancel: cancel);
  }

  Future<String?> _resolveSpeechLocaleId() async {
    try {
      final locales = await _speechToText.locales();
      if (locales.isEmpty) {
        return null;
      }
      final zhLocale = locales.firstWhere(
        (locale) =>
            locale.localeId == 'zh_CN' || locale.localeId.startsWith('zh'),
        orElse: () => locales.first,
      );
      return zhLocale.localeId;
    } catch (_) {
      return null;
    }
  }

  String _compactWakeText(String text) {
    return normalizeSpeechTranscript(
      text,
    ).toLowerCase().replaceAll(RegExp(r"""[\s,，.。!！?？、:：;；"“”'‘’]"""), '');
  }

  Set<String> _wakePhraseAliases() {
    return <String>{
      _cortanaWakePhrase,
      _defaultCortanaWakePhrase,
      '嘿 Cortana',
      'Hey Cortana',
      'Hi Cortana',
      '你好 Cortana',
      '嗨 科塔娜',
      '嘿 科塔娜',
      '嗨 小娜',
    }.map(_compactWakeText).where((text) => text.isNotEmpty).toSet();
  }

  String _stripWakePhrase(String transcript) {
    var command = normalizeSpeechTranscript(transcript);
    final configured = _cortanaWakePhrase.trim();
    if (configured.isNotEmpty) {
      command = command.replaceFirst(
        RegExp(RegExp.escape(configured), caseSensitive: false),
        '',
      );
    }
    command = command.replaceFirst(
      RegExp(
        r'(嗨|嘿|hello|hey|hi|你好)?\s*(cortana|科塔娜|小娜)\s*[,，。.!！?？、]?\s*',
        caseSensitive: false,
      ),
      '',
    );
    return normalizeSpeechTranscript(command);
  }

  String? _extractCortanaWakeCommand(String transcript) {
    final compact = _compactWakeText(transcript);
    if (compact.isEmpty) {
      return null;
    }
    for (final alias in _wakePhraseAliases()) {
      if (compact.contains(alias)) {
        return _stripWakePhrase(transcript);
      }
    }
    return null;
  }

  Future<void> _startCortanaWakeListening() async {
    if (!_canListenForCortanaWake || _speechToText.isListening) {
      return;
    }
    await _speechStopTail.catchError((_) {});
    final ready = await _ensureSystemSpeechRecognitionReady(silent: true);
    if (!ready || !_canListenForCortanaWake) {
      return;
    }
    final hasPermission = await _audioRecorder.hasPermission();
    if (!hasPermission) {
      _appendSystem('语音唤醒需要麦克风权限。');
      return;
    }

    try {
      final localeId = await _resolveSpeechLocaleId();
      _lastCortanaWakeTranscript = '';
      final started = await _speechToText.listen(
        onResult: (result) {
          final transcript = normalizeSpeechTranscript(result.recognizedWords);
          if (transcript.isEmpty ||
              transcript == _lastCortanaWakeTranscript ||
              _cortanaWakeHandling) {
            return;
          }
          _lastCortanaWakeTranscript = transcript;
          final command = _extractCortanaWakeCommand(transcript);
          if (command != null) {
            unawaited(_handleCortanaWakeDetected(command));
          }
        },
        listenFor: const Duration(minutes: 5),
        pauseFor: const Duration(seconds: 3),
        localeId: localeId,
        listenOptions: stt.SpeechListenOptions(
          listenMode: stt.ListenMode.dictation,
          partialResults: true,
          cancelOnError: false,
        ),
      );
      _cortanaWakeListening = started;
      if (started && mounted) {
        setState(() {
          _status = '语音唤醒监听中';
        });
      }
    } catch (err, stack) {
      _cortanaWakeListening = false;
      debugPrint('Cortana wake listen error: $err\n$stack');
      if (_isSpeechBusyError(err)) {
        await _stopSpeechRecognition(cancel: true);
        _scheduleCortanaWakeRestart(delay: const Duration(seconds: 2));
      } else {
        _scheduleCortanaWakeRestart(delay: const Duration(seconds: 2));
      }
    }
  }

  Future<String> _listenForCortanaWakeCommand() async {
    if (!await _ensureSystemSpeechRecognitionReady(silent: true)) {
      return '';
    }
    final completer = Completer<String>();
    var transcript = '';
    try {
      await _speechStopTail.catchError((_) {});
      final localeId = await _resolveSpeechLocaleId();
      var started = await _listenForCortanaCommandOnce(
        localeId: localeId,
        onTranscript: (text, finalResult) {
          transcript = text;
          if (finalResult && text.isNotEmpty && !completer.isCompleted) {
            completer.complete(text);
          }
        },
      );
      if (!started) {
        await _stopSpeechRecognition(cancel: true);
        started = await _listenForCortanaCommandOnce(
          localeId: localeId,
          onTranscript: (text, finalResult) {
            transcript = text;
            if (finalResult && text.isNotEmpty && !completer.isCompleted) {
              completer.complete(text);
            }
          },
        );
      }
      if (!started) {
        return '';
      }
      if (mounted) {
        setState(() {
          _status = 'Cortana 已唤醒，正在聆听...';
        });
      }
      return await Future.any<String>([
        completer.future,
        Future<String>.delayed(const Duration(seconds: 10), () => transcript),
      ]);
    } catch (err, stack) {
      debugPrint('Cortana command listen error: $err\n$stack');
      if (_isSpeechBusyError(err)) {
        await _stopSpeechRecognition(cancel: true);
      }
      return transcript;
    } finally {
      await _stopSpeechRecognition(cancel: false);
    }
  }

  Future<bool> _listenForCortanaCommandOnce({
    required String? localeId,
    required void Function(String transcript, bool finalResult) onTranscript,
  }) async {
    return await _speechToText.listen(
      onResult: (result) {
        onTranscript(
          normalizeSpeechTranscript(result.recognizedWords),
          result.finalResult,
        );
      },
      listenFor: const Duration(seconds: 10),
      pauseFor: const Duration(seconds: 2),
      localeId: localeId,
      listenOptions: stt.SpeechListenOptions(
        listenMode: stt.ListenMode.dictation,
        partialResults: true,
        cancelOnError: false,
      ),
    );
  }

  Future<void> _handleCortanaWakeDetected(String initialCommand) async {
    if (_cortanaWakeHandling) {
      return;
    }
    _cortanaWakeHandling = true;
    try {
      await _pauseCortanaWakeListening(cancel: false);
      if (!mounted) {
        return;
      }
      setState(() {
        _rootTab = RootTab.cortana;
        _cortanaFloatingMode = CortanaDisplayMode.collapsed;
        _cortanaBadge = false;
        _status = 'Cortana 已唤醒';
      });
      _triggerCortanaContextualExpression('surprised');
      _appendSystem('检测到语音唤醒词：$_cortanaWakePhrase');

      var command = normalizeSpeechTranscript(initialCommand);
      if (command.isEmpty) {
        command = normalizeSpeechTranscript(
          await _listenForCortanaWakeCommand(),
        );
      }
      if (command.isEmpty) {
        _appendSystem('已唤醒 Cortana，但未识别到有效语音内容。');
        return;
      }
      await _speakCortanaWakeCommand(command);
    } finally {
      _cortanaWakeHandling = false;
      _scheduleCortanaWakeRestart(delay: const Duration(seconds: 1));
    }
  }

  Future<void> _speakCortanaWakeCommand(String command) async {
    _appendSystem('Cortana 语音对话：$command');
    for (var attempt = 0; attempt < 10; attempt++) {
      final state = _cortanaPageKey.currentState;
      if (state != null) {
        await state.speakText(command);
        return;
      }
      await Future<void>.delayed(const Duration(milliseconds: 120));
    }
    throw StateError('Cortana 页面尚未准备好。');
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
      return null;
    }
    return Map<String, dynamic>.from(context);
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
          }).then((_) {
            debugPrint(
              '[Cortana Device Context] report sent: $locationLog',
            );
            addFlutterClientLog('定位上报成功: $locationLog');
          }).catchError((Object err, StackTrace _) {
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
    final query = _codegenSearchController.text.trim().toLowerCase();
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
    final query = _codegenSearchController.text.trim().toLowerCase();
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
      if (!mounted) {
        _deployArgsController.text = deployArgs;
        _selectedCodeProjectQualifiedName = codeProject;
        _selectedCodeTool = codeTool;
        _selectedClaudeSettings = claudeSettings;
        _selectedDeployProjectQualifiedName = deployProject;
        _selectedDeployTarget = deployTarget;
        _codegenMode = modeName == CodegenLaunchMode.deploy.name
            ? CodegenLaunchMode.deploy
            : CodegenLaunchMode.code;
        return;
      }
      setState(() {
        _deployArgsController.text = deployArgs;
        _selectedCodeProjectQualifiedName = codeProject;
        _selectedCodeTool = codeTool;
        _selectedClaudeSettings = claudeSettings;
        _selectedDeployProjectQualifiedName = deployProject;
        _selectedDeployTarget = deployTarget;
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
                            color: item.mode == CodegenLaunchMode.code
                                ? Colors.blue.withValues(alpha: 0.2)
                                : Colors.green.withValues(alpha: 0.2),
                            borderRadius: BorderRadius.circular(999),
                          ),
                          child: Text(
                            item.mode == CodegenLaunchMode.code ? '编码' : '发布',
                            style: TextStyle(
                              color: item.mode == CodegenLaunchMode.code
                                  ? Colors.blue
                                  : Colors.green,
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
                    _buildHistoryDetailRow('项目', details.projectQualifiedName),
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

  String _buildCodegenCommandPreview() {
    if (_codegenMode == CodegenLaunchMode.code) {
      final project = _selectedCodingProject;
      if (project == null) {
        return '/cg start <project@agent> <request>';
      }
      final prompt = _codegenPromptController.text.trim();
      final parts = <String>['/cg', 'start', project.qualifiedName];
      if (_selectedCodeTool.isNotEmpty) {
        parts.add('@$_selectedCodeTool');
      }
      if (_selectedClaudeSettings.isNotEmpty) {
        parts.add('--settings');
        parts.add(_selectedClaudeSettings);
      }
      if (_codegenAutoDeploy) {
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

    final command = _buildCodegenCommandPreview().trim();
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

  CortanaReplyPayload? _extractCortanaReplyPayload(ChatMessage msg) {
    if (msg.direction != MessageDirection.incoming) {
      return null;
    }
    final meta = msg.meta ?? const <String, dynamic>{};
    if (msg.messageType == 'audio') {
      final audioPath = (meta['audio_path'] ?? '').toString().trim();
      final audioBase64 = (meta['audio_base64'] ?? '').toString().trim();
      Uint8List? audioBytes;
      if (audioBase64.isNotEmpty) {
        try {
          audioBytes = base64Decode(audioBase64);
        } catch (_) {
          audioBytes = null;
        }
      }
      if (audioPath.isNotEmpty || audioBytes != null) {
        final rawActionPlan = meta['cortana_action_plan'];
        return CortanaReplyPayload(
          text: (meta['speech_text'] ?? msg.content).toString().trim(),
          audioPath: audioPath,
          audioBytes: audioBytes,
          audioFormat: (meta['audio_format'] ?? '').toString().trim(),
          actionPlan: rawActionPlan is Map
              ? Map<String, dynamic>.from(rawActionPlan)
              : null,
          requestId: (meta['cortana_request_id'] ?? '').toString().trim(),
        );
      }
    }
    if (msg.messageType == 'text' && !_isCortanaProgressMessage(msg)) {
      return CortanaReplyPayload(
        text: msg.content.trim(),
        requestId: (meta['cortana_request_id'] ?? '').toString().trim(),
      );
    }
    return null;
  }

  String _buildCortanaRequestId() {
    final now = DateTime.now().microsecondsSinceEpoch;
    final userId = _userIdController.text.trim();
    return 'cortana_${userId}_$now';
  }

  String _buildCortanaReplyKey(ChatMessage msg) {
    final audioPath = (msg.meta?['audio_path'] ?? '').toString().trim();
    final audioBase64 = (msg.meta?['audio_base64'] ?? '').toString().trim();
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
      final audioPath = (meta['audio_path'] ?? '').toString().trim();
      final audioBase64 = (meta['audio_base64'] ?? '').toString().trim();
      Uint8List? audioBytes;
      if (audioBase64.isNotEmpty) {
        try {
          audioBytes = base64Decode(audioBase64);
        } catch (_) {
          audioBytes = null;
        }
      }
      if (audioPath.isEmpty && audioBytes == null) {
        continue;
      }
      final rawActionPlan = meta['cortana_action_plan'];
      replayItems.add(
        CortanaReplayItem(
          id: _buildCortanaReplyKey(msg),
          text: (meta['speech_text'] ?? msg.content).toString().trim(),
          audioPath: audioPath,
          audioBytes: audioBytes,
          audioFormat: (meta['audio_format'] ?? '').toString().trim(),
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

  void _handleCortanaBroadcast(dynamic envelope, Map<String, dynamic> meta) {
    final broadcastText = (meta['cortana_text'] ?? envelope.content ?? '')
        .toString()
        .trim();
    if (broadcastText.isEmpty) return;

    final expression = (meta['cortana_expression'] ?? 'happy')
        .toString()
        .trim();
    final motion = (meta['cortana_motion'] ?? 'IdleWave').toString().trim();

    // 提取 TTS 音频数据
    final audioPath = (meta['cortana_audio_path'] ?? '').toString().trim();
    final audioBase64 = (meta['cortana_audio_base64'] ?? '').toString().trim();
    final audioFormat = (meta['cortana_audio_format'] ?? '').toString().trim();
    Uint8List? audioBytes;
    if (audioBase64.isNotEmpty) {
      try {
        audioBytes = base64Decode(audioBase64);
      } catch (_) {
        audioBytes = null;
      }
    }

    addFlutterClientLog('收到播报: $broadcastText');

    debugPrint(
      '[Cortana Broadcast] text=$broadcastText expr=$expression motion=$motion audioPath=${audioPath.isEmpty ? "none" : audioPath} audio=${audioBytes != null ? "${audioBytes.length}bytes" : "none"}',
    );

    final payload = CortanaReplyPayload(
      text: broadcastText,
      audioPath: audioPath,
      audioBytes: audioBytes,
      audioFormat: audioFormat,
      actionPlan: <String, dynamic>{
        'expression': expression,
        'motion': motion,
        'actions': <Map<String, dynamic>>[
          <String, dynamic>{'motion': motion, 'delay': 0},
        ],
      },
    );

    if (audioPath.isEmpty && audioBytes == null) {
      debugPrint(
        '[Cortana Broadcast] audio missing, fallback to text-only broadcast',
      );
    }

    if (!mounted) return;

    _presentCortanaFloatingBroadcast(payload);
  }

  void _presentCortanaFloatingBroadcast(CortanaReplyPayload payload) {
    setState(() {
      _cortanaBadge = true;
      if (_rootTab != RootTab.cortana &&
          _cortanaFloatingMode == CortanaDisplayMode.collapsed) {
        _cortanaFloatingMode = CortanaDisplayMode.small;
      }
    });

    debugPrint(
      '[Cortana Broadcast] raise floating cortana mode=$_cortanaFloatingMode text=${payload.text}',
    );

    if (!_cortanaAutoPlay) {
      return;
    }

    _cortanaBroadcastQueue.enqueue(payload, (nextPayload, onFinished) {
      _playQueuedCortanaBroadcast(nextPayload, onFinished);
    });
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
        if (mounted) {
          setState(() {
            _cortanaBadge = _cortanaBroadcastQueue.hasPending;
          });
        }
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
    final deviceContext =
        await _refreshCortanaDeviceContext(report: true) ??
        _currentCortanaDeviceContext();
    addFlutterClientLog('Cortana 发送: $message');
    final meta = <String, dynamic>{
      'conversation_mode': 'cortana',
      'input_mode': 'cortana_text',
      'reply_mode': 'audio_preferred',
      'cortana_request_id': requestId,
      if (deviceContext != null) 'device_context': deviceContext,
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
            if (deviceContext != null) 'device_context': deviceContext,
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

    if (anyUpdated) {
      setState(() {});
      if (lastUpdatedScopeKey == _currentScopeKey &&
          _isNearBottom(_scrollController)) {
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

  GlobalKey _messageAnchorKey(ChatMessage message) {
    final anchorId = _messageAnchorId(message);
    return _messageAnchorKeys.putIfAbsent(anchorId, GlobalKey.new);
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
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final anchorKey = _messageAnchorKey(message);
      final anchorContext = anchorKey.currentContext;
      if (anchorContext == null) {
        if (attempts >= 6) {
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
        _seenMessageIds.add(envelope.messageId);
        return;
      }

      _appendMessage(
        chatMessage,
        updateStatus: isSystemMessage ? envelope.content : 'Received message',
      );
      _recordIncomingProcessMessage(chatMessage);
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

  void _recordLlmDebugEvent(
    PushEnvelope envelope,
    Map<String, dynamic> meta,
  ) {
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
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_recording || _sending) {
      return;
    }
    await _pauseCortanaWakeListening(cancel: true);

    final hasPermission = await _audioRecorder.hasPermission();
    if (!hasPermission) {
      _appendSystem('Microphone permission denied.');
      _scheduleCortanaWakeRestart();
      return;
    }

    try {
      final tempDir = await getTemporaryDirectory();
      final useWaveFile = _isWindowsHost || _useLocalVosk;
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
      _scheduleCortanaWakeRestart();
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
    await _stopSpeechRecognition(cancel: false);

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
    await _stopSpeechRecognition(cancel: true);
    await _stopRecording(discard: true);
    _appendSystem('Voice input cancelled.');
    _scheduleCortanaWakeRestart();
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
      _scheduleCortanaWakeRestart();
    }
  }

  Future<void> _sendVoiceAsAudio() async {
    final recorded = await _stopRecording(discard: false);
    if (recorded == null) {
      _appendSystem('Voice recording unavailable.');
      _scheduleCortanaWakeRestart();
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
      final audioFormat = (_isWindowsHost || _useLocalVosk) ? 'wav' : 'm4a';
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
        await _runAuthed('Send voice audio', (client) {
          return client.sendAppMessage(
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
        });
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
    } finally {
      _scheduleCortanaWakeRestart();
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
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
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
        await _runAuthed('Send image', (client) {
          return client.sendAppMessage(
            '',
            messageType: 'image',
            meta: localMeta,
          );
        });
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
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    final text = _messageController.text.trim();
    if (text.isEmpty) {
      return;
    }
    FocusScope.of(context).unfocus();
    _messageController.clear();
    final deviceContext = await _refreshCortanaDeviceContext(
      report: true,
      force: true,
    );
    final meta = <String, dynamic>{
      if (deviceContext != null) 'device_context': deviceContext,
      if (_currentGroupId.isEmpty) ...<String, dynamic>{
        'conversation_mode': 'cortana',
        'input_mode': 'cortana_chat',
        'reply_mode': 'text',
      },
      if (_currentGroupId.isNotEmpty) ...<String, dynamic>{
        'group_id': _currentGroupId,
        'scope': 'group',
      },
    };
    _appendOutgoing(text, meta: meta.isEmpty ? null : meta);
    setState(() {
      _sending = true;
    });
    try {
      await _runAuthed('Send message', (client) {
        return client.sendAppMessage(text, meta: meta);
      });
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
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
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

  Widget _buildCortanaChatSettings() {
    final palette = _palette;
    return Container(
      decoration: BoxDecoration(
        color: palette.surfaceMuted.withValues(alpha: 0.5),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: palette.border.withValues(alpha: 0.55)),
      ),
      child: ExpansionTile(
        initiallyExpanded: _cortanaChatSettingsExpanded,
        onExpansionChanged: (v) {
          setState(() => _cortanaChatSettingsExpanded = v);
        },
        title: Text(
          'Cortana 设置',
          style: TextStyle(
            fontWeight: FontWeight.w600,
            color: palette.textPrimary,
          ),
        ),
        subtitle: Text(
          _cortanaChatSettingsExpanded ? '点击收起' : '默认折叠，点击配置',
          style: TextStyle(fontSize: 12, color: palette.textMuted),
        ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
            child: _buildCortanaChatSettingsContent(),
          ),
        ],
      ),
    );
  }

  Widget _buildCortanaChatSettingsContent() {
    final palette = _palette;
    final enabled = _cortanaEnabled;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SwitchListTile.adaptive(
          value: _cortanaEnabled,
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: const Text('启用 Cortana'),
          subtitle: const Text('允许服务端为当前账号保持 Cortana 会话'),
          onChanged: (v) =>
              _applyCortanaSettings(_cortanaSettings.copyWith(enabled: v)),
        ),
        SwitchListTile.adaptive(
          value: _cortanaAllowFullAccess,
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: const Text('允许全量感知'),
          subtitle: const Text('开放待办、锻炼、阅读、年度目标等数据'),
          onChanged: !enabled
              ? null
              : (v) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(allowFullAccess: v),
                ),
        ),
        SwitchListTile.adaptive(
          value: _cortanaAutoPlay,
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: const Text('主动播报自动播放'),
          subtitle: const Text('收到主动互动后直接语音播报'),
          onChanged: !enabled
              ? null
              : (v) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(autoPlay: v),
                ),
        ),
        SwitchListTile.adaptive(
          value: _cortanaVoiceWakeEnabled,
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: const Text('语音唤醒'),
          subtitle: Text(
            '前台持续监听"${_cortanaWakePhrase.trim().isEmpty ? '嗨 Cortana' : _cortanaWakePhrase.trim()}"后进入语音对话',
          ),
          onChanged: !enabled
              ? null
              : (v) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(voiceWakeEnabled: v),
                ),
        ),
        if (_cortanaVoiceWakeEnabled)
          TextField(
            controller: _cortanaWakePhraseCtrl,
            decoration: const InputDecoration(
              labelText: '唤醒词',
              hintText: '嗨 Cortana',
              isDense: true,
            ),
            onChanged: (v) => _applyCortanaSettings(
              _cortanaSettings.copyWith(
                wakePhrase: v.trim().isEmpty ? '嗨 Cortana' : v.trim(),
              ),
            ),
          ),
        const SizedBox(height: 6),
        DropdownButtonFormField<String>(
          initialValue: _cortanaProactiveMode,
          decoration: const InputDecoration(labelText: '主动模式', isDense: true),
          items: const [
            DropdownMenuItem(value: 'high', child: Text('High')),
            DropdownMenuItem(value: 'normal', child: Text('Normal')),
            DropdownMenuItem(value: 'low', child: Text('Low')),
          ],
          onChanged: !enabled
              ? null
              : (v) {
                  if (v == null || v.trim().isEmpty) return;
                  _applyCortanaSettings(
                    _cortanaSettings.copyWith(proactiveMode: v.trim()),
                  );
                },
        ),
        const SizedBox(height: 12),
        Text(
          '高频触发时间',
          style: TextStyle(
            color: palette.textPrimary,
            fontWeight: FontWeight.w600,
            fontSize: 14,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          '在此时间段内 Cortana 主动播报频率更高',
          style: TextStyle(color: palette.textSecondary, fontSize: 12),
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: _buildCortanaTimePicker(
                label: '开始',
                hour: _cortanaHighFreqStartHour,
                minute: _cortanaHighFreqStartMinute,
                onChanged: (h, m) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(
                    highFreqStartHour: h,
                    highFreqStartMinute: m,
                  ),
                ),
              ),
            ),
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 8),
              child: Text('—'),
            ),
            Expanded(
              child: _buildCortanaTimePicker(
                label: '结束',
                hour: _cortanaHighFreqEndHour,
                minute: _cortanaHighFreqEndMinute,
                onChanged: (h, m) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(
                    highFreqEndHour: h,
                    highFreqEndMinute: m,
                  ),
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Text(
          '人设配置',
          style: TextStyle(
            color: palette.textPrimary,
            fontWeight: FontWeight.w600,
            fontSize: 14,
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _cortanaPersonaNameCtrl,
          decoration: const InputDecoration(
            labelText: '名称',
            hintText: 'Cortana',
            isDense: true,
          ),
          onChanged: (v) => _applyCortanaSettings(
            _cortanaSettings.copyWith(personaName: v.trim()),
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _cortanaOwnerTitleCtrl,
          decoration: const InputDecoration(
            labelText: '对用户称呼',
            hintText: '主人',
            isDense: true,
          ),
          onChanged: (v) => _applyCortanaSettings(
            _cortanaSettings.copyWith(ownerTitle: v.trim()),
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _cortanaPersonaDescCtrl,
          maxLines: 3,
          decoration: const InputDecoration(
            labelText: '人设描述',
            hintText: '例如：你是一个友好、乐于助人的 AI 助手...',
            isDense: true,
            alignLabelWithHint: true,
          ),
          onChanged: (v) => _applyCortanaSettings(
            _cortanaSettings.copyWith(personaDescription: v.trim()),
          ),
        ),
      ],
    );
  }

  Widget _buildCortanaChatLogs() {
    final palette = _palette;
    final logEntries = List<FlutterClientLogEntry>.from(flutterClientLogs);
    return Container(
      decoration: BoxDecoration(
        color: palette.surfaceMuted.withValues(alpha: 0.5),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: palette.border.withValues(alpha: 0.55)),
      ),
      child: ExpansionTile(
        initiallyExpanded: _cortanaChatLogsExpanded,
        onExpansionChanged: (v) {
          setState(() => _cortanaChatLogsExpanded = v);
        },
        title: Text(
          'Cortana 运行日志',
          style: TextStyle(
            fontWeight: FontWeight.w600,
            color: palette.textPrimary,
          ),
        ),
        subtitle: Text(
          logEntries.isEmpty
              ? '暂无 Flutter 客户端日志'
              : '共 ${logEntries.length} 条 · 仅限客户端',
          style: TextStyle(fontSize: 12, color: palette.textMuted),
        ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
            child: _buildCortanaChatLogsContent(logEntries),
          ),
        ],
      ),
    );
  }

  Widget _buildCortanaChatLogsContent(List<FlutterClientLogEntry> entries) {
    final palette = _palette;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              '客户端日志',
              style: TextStyle(
                fontWeight: FontWeight.w700,
                color: palette.textPrimary,
                fontSize: 13,
              ),
            ),
            const Spacer(),
            if (entries.isNotEmpty)
              TextButton.icon(
                onPressed: () {
                  flutterClientLogs.clear();
                  if (mounted) setState(() {});
                },
                icon: const Icon(Icons.delete_outline, size: 16),
                label: const Text('清空'),
                style: TextButton.styleFrom(foregroundColor: palette.textMuted),
              ),
          ],
        ),
        const SizedBox(height: 8),
        Container(
          width: double.infinity,
          constraints: const BoxConstraints(maxHeight: 300, minHeight: 120),
          decoration: BoxDecoration(
            color: palette.surfaceMuted.withValues(alpha: 0.92),
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: palette.border.withValues(alpha: 0.55)),
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: entries.isEmpty
                ? Center(
                    child: Text(
                      '暂无日志 · 操作 Cortana 对话时将自动采集',
                      style: TextStyle(color: palette.textMuted, fontSize: 13),
                    ),
                  )
                : Scrollbar(
                    thumbVisibility: true,
                    child: ListView.builder(
                      padding: const EdgeInsets.all(10),
                      itemCount: entries.length,
                      itemBuilder: (context, index) {
                        final entry = entries[index];
                        return Padding(
                          padding: const EdgeInsets.only(bottom: 4),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                entry.timeLabel,
                                style: TextStyle(
                                  fontFamily: 'monospace',
                                  fontSize: 11,
                                  color: palette.accent,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                              const SizedBox(width: 8),
                              Expanded(
                                child: SelectableText(
                                  entry.message,
                                  style: TextStyle(
                                    fontFamily: 'monospace',
                                    fontSize: 12,
                                    height: 1.4,
                                    color: palette.textPrimary,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                  ),
          ),
        ),
      ],
    );
  }

  Widget _buildCortanaTimePicker({
    required String label,
    required int hour,
    required int minute,
    required void Function(int hour, int minute)? onChanged,
  }) {
    final hh = hour.toString().padLeft(2, '0');
    final mm = minute.toString().padLeft(2, '0');
    return InkWell(
      onTap: onChanged == null
          ? null
          : () async {
              final time = await showTimePicker(
                context: context,
                initialTime: TimeOfDay(hour: hour, minute: minute),
              );
              if (time != null) {
                onChanged(time.hour, time.minute);
              }
            },
      borderRadius: BorderRadius.circular(8),
      child: InputDecorator(
        decoration: InputDecoration(labelText: label, isDense: true),
        child: Text('$hh:$mm', style: Theme.of(context).textTheme.bodyLarge),
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
                      style: TextStyle(
                        fontSize: 12,
                        color: palette.textSecondary,
                      ),
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
                          backgroundColor: hasPartial
                              ? palette.warning
                              : palette.accent,
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
                border: Border.all(color: palette.error.withValues(alpha: 0.3)),
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
                      style: TextStyle(fontSize: 11, color: palette.error),
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
              Icon(
                Icons.palette_outlined,
                size: 18,
                color: palette.textSecondary,
              ),
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
                      style: TextStyle(
                        fontSize: 12,
                        color: palette.textSecondary,
                      ),
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

  Widget _buildChatControlsPanel() {
    final palette = _palette;
    final canLogin = !_loggingIn && !_configLoading && _clientConfig != null;
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
      width: double.infinity,
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
                            onSelected: (_) =>
                                unawaited(_switchToDirectScope()),
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
                              onSelected: (_) =>
                                  unawaited(_switchToGroupsTab()),
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
                              label: Text(
                                '${group.id} (${group.members.length})',
                              ),
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
                            OutlinedButton.icon(
                              onPressed:
                                  _sessionToken.isEmpty && _refreshToken.isEmpty
                                  ? null
                                  : _logout,
                              style: OutlinedButton.styleFrom(
                                foregroundColor: palette.textPrimary,
                                minimumSize: const Size(120, 48),
                                side: BorderSide(color: palette.borderStrong),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(18),
                                ),
                              ),
                              icon: const Icon(Icons.logout_rounded),
                              label: const Text('Logout'),
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
                                  color:
                                      (_recording
                                              ? _foregroundForColor(
                                                  palette.accent,
                                                )
                                              : palette.textPrimary)
                                          .withValues(
                                            alpha: _recording ? 1 : 0.9,
                                          ),
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
                            hintText: _currentGroupId.isEmpty ? '发消息' : '发群消息',
                            hintStyle: TextStyle(
                              color: palette.textMuted,
                              fontSize: 15,
                            ),
                            filled: true,
                            fillColor: palette.surfaceSoft,
                            floatingLabelBehavior: FloatingLabelBehavior.never,
                            isDense: true,
                            contentPadding: const EdgeInsets.symmetric(
                              horizontal: 14,
                              vertical: 10,
                            ),
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                              borderSide: BorderSide(
                                color: palette.borderStrong,
                              ),
                            ),
                            enabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                              borderSide: BorderSide(
                                color: palette.borderStrong,
                              ),
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

  Widget _buildChatBody() {
    final palette = _palette;
    return Stack(
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
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(10, 10, 10, 8),
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
                            return KeyedSubtree(
                              key: _messageAnchorKey(msg),
                              child: _MessageBubble(
                                message: msg,
                                isPlaying:
                                    _playingAudioKey ==
                                    _messagePlaybackKey(msg),
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
                              ),
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
    );
  }

  Widget _buildCodegenBody() {
    final palette = _palette;
    return CodegenBody(
      palette: CodegenBodyPalette(
        backgroundTop: palette.backgroundTop,
        surfaceMuted: palette.surfaceMuted,
        backgroundBottom: palette.backgroundBottom,
        surface: palette.surface,
        border: palette.border,
        textPrimary: palette.textPrimary,
        textSecondary: palette.textSecondary,
        error: palette.error,
      ),
      loading: _codegenLoading,
      sending: _codegenSending,
      errorText: _codegenError,
      mode: _codegenMode,
      codingProjects: _filteredCodingProjects,
      deployProjects: _filteredDeployProjects,
      selectedCodingProject: _selectedCodingProject,
      selectedDeployProject: _selectedDeployProject,
      selectedCodeTool: _selectedCodeTool,
      selectedToolSettings: _selectedClaudeSettings,
      selectedCodeToolOptions: _selectedCodingProjectTools,
      selectedToolSettingsOptions: _selectedToolSettingsOptions,
      selectedDeployTarget: _selectedDeployTarget,
      commandPreview: _buildCodegenCommandPreview(),
      autoDeploy: _codegenAutoDeploy,
      deployPackOnly: _deployPackOnly,
      codegenPromptController: _codegenPromptController,
      codegenSearchController: _codegenSearchController,
      deployArgsController: _deployArgsController,
      history: _codegenHistory,
      onRefresh: () => unawaited(_loadCodegenProjects()),
      onModeChanged: _setCodegenMode,
      onSearchChanged: (_) {
        setState(() {
          _syncFilteredCodegenSelections();
        });
      },
      onCodeProjectChanged: _handleCodeProjectChanged,
      onCodeToolChanged: _handleCodeToolChanged,
      onToolSettingsChanged: _handleClaudeSettingsChanged,
      onPromptChanged: (_) => setState(() {}),
      onAutoDeployChanged: (value) {
        setState(() {
          _codegenAutoDeploy = value;
        });
        unawaited(_persistCodegenPreferences());
      },
      onDeployProjectChanged: _handleDeployProjectChanged,
      onDeployTargetChanged: _handleDeployTargetChanged,
      onDeployPackOnlyChanged: (value) {
        setState(() {
          _deployPackOnly = _selectedDeployProject?.buildOnly == true
              ? true
              : value;
        });
        unawaited(_persistCodegenPreferences());
      },
      onDeployArgsChanged: (_) {
        setState(() {});
        unawaited(_persistCodegenPreferences());
      },
      onSend: _sendCodegenCommand,
      onClearHistory: () {
        setState(() {
          _codegenHistory.removeWhere((item) => !item.locked);
        });
        unawaited(_persistCodegenPreferences());
      },
      onShowHistoryDetails: (item) =>
          unawaited(_showCodegenHistoryDetails(item)),
      onReExecute: (item) => _reExecuteCodegenHistory(item),
      onToggleLock: (item) => _toggleCodegenHistoryLock(item),
    );
  }

  Widget _buildSettingsBody() {
    final palette = _palette;
    final baseUrl = _clientConfig?.baseUrl ?? '';
    final receiveToken = _clientConfig?.receiveToken ?? '';

    return Container(
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
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(10, 10, 10, 10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
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
              const SizedBox(height: 12),
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
                      foregroundColor: _foregroundForColor(palette.accent),
                      minimumSize: const Size(0, 48),
                      padding: const EdgeInsets.symmetric(horizontal: 14),
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
              const SizedBox(height: 8),
              _buildCortanaChatSettings(),
              const SizedBox(height: 8),
              _buildCortanaChatLogs(),
            ],
          ),
        ),
      ),
    );
  }

  void _selectRootTab(RootTab nextTab) {
    setState(() {
      _rootTab = nextTab;
      _sidebarExpanded = false;
    });
    if (nextTab == RootTab.cortana) {
      _reportCortanaUiEvent('cortana_tab_open', summary: '用户切换到了 Cortana 页签');
    }
    if (nextTab == RootTab.codegen) {
      _triggerCortanaContextualExpression('surprised');
      if (_codingProjects.isEmpty &&
          _deployProjects.isEmpty &&
          !_codegenLoading &&
          _sessionToken.isNotEmpty) {
        unawaited(_loadCodegenProjects());
      }
    }
  }

  Widget _buildSidebarNavButton({
    required IconData icon,
    required String label,
    required bool selected,
    required VoidCallback onPressed,
  }) {
    final palette = _palette;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      child: Material(
        color: selected
            ? palette.accent.withValues(alpha: 0.18)
            : Colors.transparent,
        borderRadius: BorderRadius.circular(16),
        child: InkWell(
          onTap: onPressed,
          borderRadius: BorderRadius.circular(16),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
            child: Row(
              children: [
                Icon(
                  icon,
                  color: selected ? palette.accent : palette.textPrimary,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    label,
                    style: TextStyle(
                      color: selected ? palette.accent : palette.textPrimary,
                      fontWeight: selected ? FontWeight.w800 : FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildAppSidebar() {
    final palette = _palette;
    final width = math.min(
      math.max(MediaQuery.sizeOf(context).width - 24, 240.0),
      316.0,
    );
    return Container(
      width: width,
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: <Color>[palette.surface, palette.surfaceRaised],
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
        ),
        border: Border(right: BorderSide(color: palette.border)),
        boxShadow: [
          BoxShadow(
            blurRadius: 18,
            color: Colors.black.withValues(alpha: 0.18),
            offset: const Offset(10, 0),
          ),
        ],
      ),
      child: SafeArea(
        bottom: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 10, 10, 6),
              child: Row(
                children: [
                  Icon(Icons.view_sidebar_outlined, color: palette.textPrimary),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      'Sidebar',
                      style: TextStyle(
                        color: palette.textPrimary,
                        fontWeight: FontWeight.w800,
                        fontSize: 16,
                      ),
                    ),
                  ),
                  IconButton(
                    tooltip: '关闭侧边栏',
                    onPressed: () {
                      setState(() => _sidebarExpanded = false);
                    },
                    icon: const Icon(Icons.close_rounded),
                  ),
                ],
              ),
            ),
            _buildSidebarNavButton(
              icon: Icons.chat_bubble_outline_rounded,
              label: '聊天',
              selected: _rootTab == RootTab.chat,
              onPressed: () => _selectRootTab(RootTab.chat),
            ),
            _buildSidebarNavButton(
              icon: Icons.terminal_outlined,
              label: '编码发布',
              selected: _rootTab == RootTab.codegen,
              onPressed: () => _selectRootTab(RootTab.codegen),
            ),
            _buildSidebarNavButton(
              icon: Icons.face_outlined,
              label: 'Cortana',
              selected: _rootTab == RootTab.cortana,
              onPressed: () => _selectRootTab(RootTab.cortana),
            ),
            _buildSidebarNavButton(
              icon: Icons.bug_report_outlined,
              label: '调试',
              selected: _rootTab == RootTab.debug,
              onPressed: () => _selectRootTab(RootTab.debug),
            ),
            _buildSidebarNavButton(
              icon: Icons.settings_outlined,
              label: '设置',
              selected: _rootTab == RootTab.settings,
              onPressed: () => _selectRootTab(RootTab.settings),
            ),
            Divider(color: palette.border, height: 18),
            if (_rootTab == RootTab.chat)
              Expanded(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(10, 0, 10, 16),
                  child: _buildChatControlsPanel(),
                ),
              )
            else
              Padding(
                padding: const EdgeInsets.fromLTRB(18, 8, 18, 0),
                child: Text(
                  'Direct、Groups 和 Controls 已集中到聊天侧边栏。',
                  style: TextStyle(
                    color: palette.textSecondary,
                    fontSize: 12,
                    height: 1.35,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Timer? _cortanaExpressionTimer;

  void _triggerCortanaContextualExpression(String expression) {
    _cortanaExpressionTimer?.cancel();
    setState(() {
      _cortanaContextualExpression = expression;
    });
    _cortanaExpressionTimer = Timer(const Duration(seconds: 3), () {
      if (!mounted) return;
      setState(() {
        _cortanaContextualExpression = null;
      });
    });
  }

  Widget _buildCortanaLayer() {
    final isFullscreen = _rootTab == RootTab.cortana;
    final floatingBottomInset = _rootTab == RootTab.chat ? 116.0 : 28.0;
    return CortanaPage(
      key: _cortanaPageKey,
      mode: isFullscreen ? CortanaDisplayMode.fullscreen : _cortanaFloatingMode,
      onSendMessage: _sendCortanaMessage,
      externalVoiceHistory: _buildCortanaReplayHistory(),
      contextualExpression: _cortanaContextualExpression,
      showBadge: _cortanaBadge,
      autoCollapseDelay: const Duration(seconds: 8),
      floatingBottomInset: floatingBottomInset,
      onTapWhenFloating: () {
        setState(() {
          _cortanaBadge = false;
          _cortanaFloatingMode =
              _cortanaFloatingMode == CortanaDisplayMode.collapsed
              ? CortanaDisplayMode.small
              : _cortanaFloatingMode;
        });
        _reportCortanaUiEvent(
          'cortana_floating_tap',
          summary: '用户点击了 Cortana 浮动按钮并展开小窗',
        );
      },
      onLongPressWhenFloating: () {
        setState(() {
          _cortanaBadge = false;
          _rootTab = RootTab.cortana;
          _cortanaFloatingMode = CortanaDisplayMode.collapsed;
        });
        _reportCortanaUiEvent(
          'cortana_floating_long_press',
          summary: '用户长按 Cortana 浮动按钮并进入页签',
        );
      },
      onModeChanged: (mode) {
        setState(() {
          _cortanaFloatingMode = mode;
          if (mode == CortanaDisplayMode.collapsed) {
            _cortanaBadge = false;
          }
        });
      },
      settings: _cortanaSettings,
      onSettingsChanged: _applyCortanaSettings,
    );
  }

  Widget _buildDebugBody() {
    final palette = _palette;
    final events = _llmDebugEvents.reversed.toList(growable: false);
    return SafeArea(
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 86, 16, 10),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    'LLM 调试',
                    style: TextStyle(
                      color: palette.textPrimary,
                      fontSize: 20,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
                IconButton(
                  tooltip: '清空',
                  onPressed: _llmDebugEvents.isEmpty
                      ? null
                      : () => setState(_llmDebugEvents.clear),
                  icon: const Icon(Icons.delete_sweep_outlined),
                ),
              ],
            ),
          ),
          Expanded(
            child: events.isEmpty
                ? Center(
                    child: Text(
                      '暂无调试事件',
                      style: TextStyle(color: palette.textSecondary),
                    ),
                  )
                : ListView.separated(
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 24),
                    itemCount: events.length,
                    separatorBuilder: (_, _) => const SizedBox(height: 10),
                    itemBuilder: (context, index) =>
                        _buildDebugEventTile(events[index]),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildDebugEventTile(LlmDebugEvent event) {
    final palette = _palette;
    final payload = event.payload;
    final summary = _debugEventSummary(event, payload);
    return Material(
      color: palette.surfaceRaised.withValues(alpha: 0.92),
      borderRadius: BorderRadius.circular(8),
      child: ExpansionTile(
        tilePadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
        childrenPadding: const EdgeInsets.fromLTRB(14, 0, 14, 14),
        leading: Icon(_debugEventIcon(event.event), color: palette.accent),
        title: Text(
          event.label,
          style: TextStyle(
            color: palette.textPrimary,
            fontWeight: FontWeight.w800,
          ),
        ),
        subtitle: Text(
          '${_formatTime(event.timestamp)}  $summary',
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(color: palette.textSecondary, height: 1.3),
        ),
        children: [
          Align(
            alignment: Alignment.centerLeft,
            child: SelectableText(
              event.detailText,
              style: TextStyle(
                color: palette.textPrimary,
                fontFamily: 'monospace',
                fontSize: 12,
                height: 1.35,
              ),
            ),
          ),
        ],
      ),
    );
  }

  String _debugEventSummary(
    LlmDebugEvent event,
    Map<String, dynamic> payload,
  ) {
    if (event.event == 'debug_prompt') {
      return 'system ${payload['system_prompt_chars'] ?? 0} chars · tools ${payload['visible_tools_count'] ?? 0}';
    }
    if (event.event == 'debug_llm_round') {
      return '第 ${payload['iteration'] ?? '?'} 轮 · ${payload['duration'] ?? ''} · tool_calls ${payload['tool_calls_count'] ?? 0}';
    }
    return event.content.replaceAll('\n', ' ');
  }

  IconData _debugEventIcon(String event) {
    switch (event) {
      case 'debug_prompt':
        return Icons.article_outlined;
      case 'debug_llm_round':
        return Icons.psychology_alt_outlined;
      case 'tool_call':
      case 'tool_result':
      case 'tool_progress':
        return Icons.build_circle_outlined;
      case 'task_complete':
        return Icons.check_circle_outline;
      default:
        return Icons.notes_outlined;
    }
  }

  String _formatTime(DateTime time) {
    final hh = time.hour.toString().padLeft(2, '0');
    final mm = time.minute.toString().padLeft(2, '0');
    final ss = time.second.toString().padLeft(2, '0');
    return '$hh:$mm:$ss';
  }

  @override
  Widget build(BuildContext context) {
    final palette = _palette;
    return Scaffold(
      extendBodyBehindAppBar: true,
      appBar: AppBar(
        leading: IconButton(
          tooltip: '打开侧边栏',
          onPressed: () {
            setState(() => _sidebarExpanded = true);
          },
          icon: const Icon(Icons.menu_rounded),
        ),
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
              _rootTab == RootTab.chat
                  ? (_currentGroupId.isEmpty
                        ? 'Direct conversation'
                        : 'Group ${_currentGroupId.toLowerCase()}')
                  : _rootTab == RootTab.codegen
                  ? 'Fast path for /cg start and /cg deploy'
                  : _rootTab == RootTab.cortana
                  ? 'Live2D Assistant'
                  : _rootTab == RootTab.debug
                  ? 'LLM prompt and tool trace'
                  : 'App Settings',
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w500,
                color: palette.textSecondary,
              ),
            ),
          ],
        ),
        actions: [
          if (_rootTab == RootTab.cortana)
            IconButton(
              tooltip: '播报历史',
              onPressed: _openCortanaHistoryPage,
              icon: const Icon(Icons.history_rounded),
            ),
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
          Stack(
            children: [
              Offstage(
                offstage: _rootTab != RootTab.chat,
                child: TickerMode(
                  enabled: _rootTab == RootTab.chat,
                  child: _buildChatBody(),
                ),
              ),
              Offstage(
                offstage: _rootTab != RootTab.codegen,
                child: TickerMode(
                  enabled: _rootTab == RootTab.codegen,
                  child: _buildCodegenBody(),
                ),
              ),
              Offstage(
                offstage: _rootTab != RootTab.settings,
                child: TickerMode(
                  enabled: _rootTab == RootTab.settings,
                  child: _buildSettingsBody(),
                ),
              ),
              Offstage(
                offstage: _rootTab != RootTab.debug,
                child: TickerMode(
                  enabled: _rootTab == RootTab.debug,
                  child: _buildDebugBody(),
                ),
              ),
              _buildCortanaLayer(),
            ],
          ),
          if (_sidebarExpanded) ...[
            Positioned.fill(
              child: GestureDetector(
                onTap: () {
                  setState(() => _sidebarExpanded = false);
                },
                child: ColoredBox(
                  color: Colors.black.withValues(alpha: 0.28),
                ),
              ),
            ),
            Positioned(top: 0, bottom: 0, left: 0, child: _buildAppSidebar()),
          ],
        ],
      ),
      bottomNavigationBar: _rootTab == RootTab.settings
          ? null
          : NavigationBar(
              selectedIndex: switch (_rootTab) {
                RootTab.chat => 0,
                RootTab.codegen => 1,
                RootTab.cortana => 2,
                RootTab.debug => 3,
                RootTab.settings => 0,
              },
              onDestinationSelected: (index) {
                final nextTab = switch (index) {
                  0 => RootTab.chat,
                  1 => RootTab.codegen,
                  2 => RootTab.cortana,
                  _ => RootTab.debug,
                };
                _selectRootTab(nextTab);
              },
              destinations: const [
                NavigationDestination(
                  icon: Icon(Icons.chat_bubble_outline_rounded),
                  selectedIcon: Icon(Icons.chat_bubble_rounded),
                  label: '聊天',
                ),
                NavigationDestination(
                  icon: Icon(Icons.terminal_outlined),
                  selectedIcon: Icon(Icons.terminal_rounded),
                  label: '编码发布',
                ),
                NavigationDestination(
                  icon: Icon(Icons.face_outlined),
                  selectedIcon: Icon(Icons.face_rounded),
                  label: 'Cortana',
                ),
                NavigationDestination(
                  icon: Icon(Icons.bug_report_outlined),
                  selectedIcon: Icon(Icons.bug_report_rounded),
                  label: '调试',
                ),
              ],
            ),
    );
  }
}

class _MessageBubble extends StatefulWidget {
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
  State<_MessageBubble> createState() => _MessageBubbleState();
}

class _MessageBubbleState extends State<_MessageBubble> {
  static const Duration _typewriterTick = Duration(milliseconds: 18);
  static const int _kCollapseThreshold = 400;
  Timer? _typewriterTimer;
  int _visibleChars = 0;
  bool _expanded = false;
  bool _userToggledExpand = false;

  ChatMessage get message => widget.message;

  String get _videoPath =>
      (message.meta?['file_path'] ?? message.meta?['video_path'] ?? '')
          .toString()
          .trim();

  bool get _shouldAnimateStreamText {
    // 流式 codegen 消息频繁覆盖同一条记录，逐字动画会持续改变气泡高度，
    // 再叠加自动滚动，容易出现页面上下抖动。
    return false;
  }

  String get _displayText {
    final fullText = _resolvedText;
    if (!_shouldAnimateStreamText) {
      return fullText;
    }
    final end = _visibleChars.clamp(0, fullText.length);
    return fullText.substring(0, end);
  }

  String get _resolvedText {
    if (!isApkChatMessage(message)) {
      return message.content;
    }
    final version = extractApkVersion(message);
    final versionLine = version != null ? '\n版本: $version' : '';
    return '${message.content}$versionLine\n点击安装 APK';
  }

  @override
  void initState() {
    super.initState();
    _syncTypewriter(forceFullText: false);
  }

  @override
  void didUpdateWidget(covariant _MessageBubble oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.message.content != widget.message.content ||
        oldWidget.message.meta != widget.message.meta ||
        oldWidget.message.messageType != widget.message.messageType) {
      if (!_userToggledExpand) {
        _expanded = false;
      }
      _syncTypewriter(forceFullText: false);
    }
  }

  @override
  void dispose() {
    _typewriterTimer?.cancel();
    super.dispose();
  }

  void _syncTypewriter({required bool forceFullText}) {
    _typewriterTimer?.cancel();
    final fullText = _resolvedText;
    if (!_shouldAnimateStreamText || fullText.isEmpty) {
      _visibleChars = fullText.length;
      return;
    }
    if (forceFullText) {
      _visibleChars = fullText.length;
      return;
    }
    if (_visibleChars > fullText.length) {
      _visibleChars = fullText.length;
    }
    if (_visibleChars == 0) {
      _visibleChars = math.min(1, fullText.length);
    }
    if (_visibleChars >= fullText.length) {
      return;
    }
    _typewriterTimer = Timer.periodic(_typewriterTick, (timer) {
      if (!mounted) {
        timer.cancel();
        return;
      }
      final latestText = _resolvedText;
      if (_visibleChars >= latestText.length) {
        timer.cancel();
        return;
      }
      setState(() {
        final remaining = latestText.length - _visibleChars;
        final step = remaining > 180
            ? 8
            : remaining > 80
            ? 4
            : remaining > 24
            ? 2
            : 1;
        _visibleChars = math.min(latestText.length, _visibleChars + step);
      });
    });
  }

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
    final fgColor = isOutgoing
        ? _foregroundForColor(bgColor)
        : palette.textPrimary;
    final isAudio = message.messageType == 'audio';
    final isImage = message.messageType == 'image';
    final isVideo = message.messageType == 'video';
    final isApk = isApkChatMessage(message);
    final durationMs = message.meta?['duration_ms'];
    final durationText = durationMs is num
        ? '${(durationMs / 1000).toStringAsFixed(1)}s'
        : '';
    final imageBase64 = (message.meta?['image_base64'] ?? '').toString().trim();
    final displayText = _displayText;
    final textOverflows =
        !isAudio && !isImage && !isVideo && displayText.length > _kCollapseThreshold;
    final collapsedText = textOverflows && !_expanded
        ? '${displayText.substring(0, _kCollapseThreshold)}...'
        : displayText;
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
          onTap: (isAudio || isApk || (isVideo && _videoPath.isEmpty))
              ? () => widget.onTap()
              : textOverflows
              ? () => setState(() {
                  _expanded = !_expanded;
                  _userToggledExpand = true;
                })
              : null,
          onLongPress: () => widget.onCopy(),
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
                          displayText,
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
                        widget.isPlaying
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
                if (isVideo)
                  _VideoMessagePreview(
                    path: _videoPath,
                    caption: displayText,
                    foregroundColor: isSystem ? palette.textSecondary : fgColor,
                    surfaceColor: isOutgoing
                        ? accentForeground.withValues(alpha: 0.12)
                        : palette.surfaceSoft,
                  ),
                if (!isAudio && !isImage && !isVideo) ...[
                  Text(
                    collapsedText,
                    style: TextStyle(
                      color: isSystem ? palette.textSecondary : fgColor,
                      height: 1.35,
                    ),
                  ),
                  if (textOverflows) ...[
                    const SizedBox(height: 4),
                    Text(
                      _expanded ? '点击收起' : '点击展开',
                      style: TextStyle(
                        color: palette.accent,
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ],
                const SizedBox(height: 6),
                Text(
                  isImage
                      ? '${_formatTime(message.timestamp)}  Long press to copy'
                      : isVideo
                      ? '${_formatTime(message.timestamp)}  Tap video to play · Long press to copy'
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

class _VideoMessagePreview extends StatelessWidget {
  const _VideoMessagePreview({
    required this.path,
    required this.caption,
    required this.foregroundColor,
    required this.surfaceColor,
  });

  final String path;
  final String caption;
  final Color foregroundColor;
  final Color surfaceColor;

  @override
  Widget build(BuildContext context) {
    final caption = this.caption.trim();
    final path = this.path.trim();
    if (path.isEmpty) {
      return _VideoPlaceholder(
        text: caption.isEmpty ? 'Video ready to download' : caption,
        icon: Icons.download_rounded,
        foregroundColor: foregroundColor,
        surfaceColor: surfaceColor,
      );
    }

    final videoUri = Uri.file(path).toString();
    final html = '''
<!doctype html>
<html>
<head>
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    html, body { margin: 0; padding: 0; background: #000; overflow: hidden; }
    video { width: 100vw; height: 100vh; object-fit: contain; background: #000; }
  </style>
</head>
<body>
  <video controls playsinline preload="metadata" src="$videoUri"></video>
</body>
</html>
''';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(14),
          child: AspectRatio(
            aspectRatio: 16 / 9,
            child: InAppWebView(
              initialData: InAppWebViewInitialData(data: html),
              initialSettings: InAppWebViewSettings(
                allowsInlineMediaPlayback: true,
                mediaPlaybackRequiresUserGesture: true,
                allowFileAccessFromFileURLs: true,
                allowUniversalAccessFromFileURLs: true,
              ),
            ),
          ),
        ),
        if (caption.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(
            caption,
            style: TextStyle(color: foregroundColor, height: 1.35),
          ),
        ],
      ],
    );
  }
}

class _VideoPlaceholder extends StatelessWidget {
  const _VideoPlaceholder({
    required this.text,
    required this.icon,
    required this.foregroundColor,
    required this.surfaceColor,
  });

  final String text;
  final IconData icon;
  final Color foregroundColor;
  final Color surfaceColor;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(minHeight: 140),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: surfaceColor,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: [
          Icon(icon, color: foregroundColor, size: 24),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              text,
              style: TextStyle(color: foregroundColor, height: 1.35),
            ),
          ),
        ],
      ),
    );
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
                        final height = _baseHeights[i] + _amplitudes[i] * wave;
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
