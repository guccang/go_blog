part of 'cortana_page.dart';

class CortanaPage extends StatefulWidget {
  const CortanaPage({
    super.key,
    this.mode = CortanaDisplayMode.fullscreen,
    this.onSendMessage,
    this.externalVoiceHistory = const <CortanaReplayItem>[],
    this.onTapWhenFloating,
    this.onLongPressWhenFloating,
    this.onModeChanged,
    this.contextualExpression,
    this.showBadge = false,
    this.voiceWakeListening = false,
    this.awaitingVoiceCommand = false,
    this.autoCollapseDelay = const Duration(seconds: 8),
    this.floatingBottomInset = 0,
    this.onBroadcast,
    this.settings = const CortanaSettings(),
    this.onSettingsChanged,
    this.live2dModelUrl = '',
    this.viewTransform = CortanaModelViewTransform.defaults,
    this.onViewTransformChanged,
    this.onImmersiveUiChanged,
  });

  final CortanaDisplayMode mode;
  final Future<CortanaReplyPayload> Function(String message)? onSendMessage;
  final List<CortanaReplayItem> externalVoiceHistory;
  final VoidCallback? onTapWhenFloating;
  final VoidCallback? onLongPressWhenFloating;
  final ValueChanged<CortanaDisplayMode>? onModeChanged;
  final String? contextualExpression;
  final bool showBadge;
  final bool voiceWakeListening;
  final bool awaitingVoiceCommand;
  final Duration autoCollapseDelay;
  final double floatingBottomInset;
  final void Function(CortanaReplyPayload payload)? onBroadcast;
  final CortanaSettings settings;
  final ValueChanged<CortanaSettings>? onSettingsChanged;
  final String live2dModelUrl;
  final CortanaModelViewTransform viewTransform;
  final ValueChanged<CortanaModelViewTransform>? onViewTransformChanged;
  final ValueChanged<bool>? onImmersiveUiChanged;

  @override
  State<CortanaPage> createState() => CortanaPageState();
}
