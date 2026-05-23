part of '../../main.dart';

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
