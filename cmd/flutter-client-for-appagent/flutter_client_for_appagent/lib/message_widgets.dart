part of 'main.dart';

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
    final isCodegenTask = isCodegenPreviewChatMessage(message);
    final durationMs = message.meta?['duration_ms'];
    final durationText = durationMs is num
        ? '${(durationMs / 1000).toStringAsFixed(1)}s'
        : '';
    final imagePath = isImage ? _messageImagePath(message) : '';
    final imageBase64 = (message.meta?['image_base64'] ?? '').toString().trim();
    final imageKey = _messageImageCacheKey(message, imageBase64);
    final displayText = _displayText;
    final textOverflows =
        !isAudio &&
        !isImage &&
        !isVideo &&
        displayText.length > _kCollapseThreshold;
    final collapsedText = textOverflows && !_expanded
        ? '${displayText.substring(0, _kCollapseThreshold)}...'
        : displayText;
    Uint8List? imageBytes;
    if (isImage && imagePath.isEmpty && imageBase64.isNotEmpty) {
      imageBytes = _MessageImageBytesCache.instance.get(imageKey, imageBase64);
    }
    final devicePixelRatio = MediaQuery.devicePixelRatioOf(context);
    final imageCacheWidth = ((bubbleMaxWidth - 28) * devicePixelRatio)
        .clamp(320.0, 1600.0)
        .round();

    return RepaintBoundary(
      child: Align(
        alignment: alignment,
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: bubbleMaxWidth),
          child: InkWell(
            onTap:
                (isAudio ||
                    isApk ||
                    isCodegenTask ||
                    (isVideo && _videoPath.isEmpty))
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
                        if (imagePath.isNotEmpty)
                          ClipRRect(
                            borderRadius: BorderRadius.circular(14),
                            child: ConstrainedBox(
                              constraints: BoxConstraints(
                                maxWidth: bubbleMaxWidth - 28,
                                maxHeight: 360,
                              ),
                              child: Image.file(
                                File(imagePath),
                                fit: BoxFit.cover,
                                gaplessPlayback: true,
                                cacheWidth: imageCacheWidth,
                                filterQuality: FilterQuality.low,
                                errorBuilder: (context, error, stackTrace) {
                                  return _UnavailableImageBox(
                                    foregroundColor: isSystem
                                        ? palette.textSecondary
                                        : fgColor,
                                    surfaceColor: isOutgoing
                                        ? accentForeground.withValues(
                                            alpha: 0.12,
                                          )
                                        : palette.surfaceSoft,
                                  );
                                },
                              ),
                            ),
                          )
                        else if (imageBytes != null)
                          ClipRRect(
                            borderRadius: BorderRadius.circular(14),
                            child: ConstrainedBox(
                              constraints: BoxConstraints(
                                maxWidth: bubbleMaxWidth - 28,
                                maxHeight: 360,
                              ),
                              child: Image.memory(
                                imageBytes,
                                fit: BoxFit.cover,
                                gaplessPlayback: true,
                                cacheWidth: imageCacheWidth,
                                filterQuality: FilterQuality.low,
                              ),
                            ),
                          )
                        else
                          _UnavailableImageBox(
                            foregroundColor: isSystem
                                ? palette.textSecondary
                                : fgColor,
                            surfaceColor: isOutgoing
                                ? accentForeground.withValues(alpha: 0.12)
                                : palette.surfaceSoft,
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
                      foregroundColor: isSystem
                          ? palette.textSecondary
                          : fgColor,
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
                        : isCodegenTask
                        ? '${_formatTime(message.timestamp)}  点击查看实时预览 · 长按复制'
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

String _messageImageCacheKey(ChatMessage message, String imageBase64) {
  final stableParts = <Object?>[
    message.meta?['_message_id'],
    message.meta?['file_id'],
    message.meta?['object_key'],
    message.meta?['image_path'],
    message.meta?['file_path'],
  ];
  for (final part in stableParts) {
    final value = (part ?? '').toString().trim();
    if (value.isNotEmpty) {
      return '${message.scopeKey}|${message.messageType}|$value';
    }
  }
  return '${message.scopeKey}|${message.timestamp.microsecondsSinceEpoch}|'
      '${imageBase64.length}|${imageBase64.hashCode}';
}

String _messageImagePath(ChatMessage message) {
  final meta = message.meta;
  if (meta == null) {
    return '';
  }
  for (final key in const <String>['image_path', 'file_path']) {
    final value = (meta[key] ?? '').toString().trim();
    if (value.isNotEmpty) {
      return value;
    }
  }
  return '';
}

class _MessageImageBytesCache {
  _MessageImageBytesCache._();

  static final _MessageImageBytesCache instance = _MessageImageBytesCache._();
  static const int _maxEntries = 24;
  static const int _maxBytes = 24 * 1024 * 1024;

  final LinkedHashMap<String, Uint8List> _items =
      LinkedHashMap<String, Uint8List>();
  int _bytes = 0;

  Uint8List? get(String key, String imageBase64) {
    final existing = _items.remove(key);
    if (existing != null) {
      _items[key] = existing;
      return existing;
    }
    try {
      final decoded = base64Decode(imageBase64);
      _items[key] = decoded;
      _bytes += decoded.lengthInBytes;
      _evictIfNeeded();
      return decoded;
    } catch (_) {
      return null;
    }
  }

  void _evictIfNeeded() {
    while (_items.length > _maxEntries || _bytes > _maxBytes) {
      final oldestKey = _items.isEmpty ? null : _items.keys.first;
      if (oldestKey == null) {
        _bytes = 0;
        return;
      }
      final removed = _items.remove(oldestKey);
      if (removed != null) {
        _bytes -= removed.lengthInBytes;
      }
    }
  }
}

class _UnavailableImageBox extends StatelessWidget {
  const _UnavailableImageBox({
    required this.foregroundColor,
    required this.surfaceColor,
  });

  final Color foregroundColor;
  final Color surfaceColor;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: surfaceColor,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        'Image unavailable',
        style: TextStyle(color: foregroundColor),
      ),
    );
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
    final html =
        '''
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
          Text(caption, style: TextStyle(color: foregroundColor, height: 1.35)),
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
