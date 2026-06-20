bool shouldDedupeIncomingMessageId({
  required String origin,
  required String messageId,
  required Set<String> seenMessageIds,
}) {
  final normalizedMessageId = messageId.trim();
  if (normalizedMessageId.isEmpty) {
    return false;
  }
  if (origin.trim() == 'codegen-stream') {
    return false;
  }
  return seenMessageIds.contains(normalizedMessageId);
}
