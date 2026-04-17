import 'dart:async';
import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart';

class CortanaPage extends StatefulWidget {
  const CortanaPage({super.key});

  @override
  State<CortanaPage> createState() => _CortanaPageState();
}

class _CortanaPageState extends State<CortanaPage> {
  InAppWebViewController? _webCtrl;
  final TextEditingController _textCtrl = TextEditingController();
  final AudioPlayer _audio = AudioPlayer();
  Timer? _lipTimer;
  bool _speaking = false;

  static const _expressions = ['happy', 'sad', 'surprised'];
  static const _motions = [('TapBody', 0), ('Idle', 0)];
  static const _costumes = ['default', 'casual', 'formal'];

  @override
  void dispose() {
    _lipTimer?.cancel();
    _audio.dispose();
    _textCtrl.dispose();
    super.dispose();
  }

  Future<void> _callJS(String js) async {
    await _webCtrl?.evaluateJavascript(source: js);
  }

  Future<void> _speak(String text) async {
    if (text.isEmpty || _speaking) return;
    setState(() => _speaking = true);

    // 启动口型同步定时器
    _lipTimer = Timer.periodic(const Duration(milliseconds: 120), (_) {
      final amp = 0.3 + (0.7 * (DateTime.now().millisecond % 100) / 100);
      _callJS('startLipSync($amp)');
    });

    // 用系统TTS或audioplayers播放（此处用简单延时模拟，实际接入TTS接口）
    await Future.delayed(Duration(milliseconds: text.length * 80));

    _lipTimer?.cancel();
    await _callJS('stopLipSync()');
    setState(() => _speaking = false);
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Column(
      children: [
        Expanded(
          child: InAppWebView(
            initialFile: 'assets/cortana/index.html',
            initialSettings: InAppWebViewSettings(
              transparentBackground: true,
              allowFileAccessFromFileURLs: true,
              allowUniversalAccessFromFileURLs: true,
            ),
            onWebViewCreated: (ctrl) => _webCtrl = ctrl,
          ),
        ),
        Container(
          color: cs.surface,
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              // 表情行
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: Row(
                  children: [
                    for (final e in _expressions)
                      Padding(
                        padding: const EdgeInsets.only(right: 6),
                        child: ActionChip(
                          label: Text(e),
                          onPressed: () => _callJS("setExpression('$e')"),
                        ),
                      ),
                    const SizedBox(width: 8),
                    for (final m in _motions)
                      Padding(
                        padding: const EdgeInsets.only(right: 6),
                        child: ActionChip(
                          label: Text(m.$1),
                          avatar: const Icon(Icons.directions_run, size: 14),
                          onPressed: () => _callJS("setMotion('${m.$1}', ${m.$2})"),
                        ),
                      ),
                    const SizedBox(width: 8),
                    for (final c in _costumes)
                      Padding(
                        padding: const EdgeInsets.only(right: 6),
                        child: ActionChip(
                          label: Text(c),
                          avatar: const Icon(Icons.checkroom, size: 14),
                          onPressed: () => _callJS("setCostume('$c')"),
                        ),
                      ),
                  ],
                ),
              ),
              const SizedBox(height: 8),
              // 说话输入行
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _textCtrl,
                      decoration: const InputDecoration(
                        hintText: '输入让 Cortana 说的话...',
                        isDense: true,
                        border: OutlineInputBorder(),
                        contentPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  FilledButton.icon(
                    onPressed: _speaking ? null : () => _speak(_textCtrl.text.trim()),
                    icon: _speaking
                        ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Icon(Icons.record_voice_over),
                    label: Text(_speaking ? '说话中' : '说话'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}
