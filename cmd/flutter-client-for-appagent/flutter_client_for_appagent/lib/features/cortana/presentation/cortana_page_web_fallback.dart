part of 'cortana_page.dart';

extension _CortanaPageStateWebFallback on CortanaPageState {
  Widget _buildWebCortanaBackdrop(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return DecoratedBox(
      decoration: BoxDecoration(color: Colors.transparent),
      child: Center(
        child: Icon(
          Icons.face_rounded,
          size: 96,
          color: cs.primary.withValues(alpha: 0.42),
        ),
      ),
    );
  }
}
