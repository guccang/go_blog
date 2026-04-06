import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_client_for_appagent/main.dart';

void main() {
  testWidgets('renders app-agent client shell', (tester) async {
    await tester.pumpWidget(const AppAgentClientApp());

    expect(find.text('App Agent'), findsOneWidget);
    expect(find.text('Direct conversation'), findsOneWidget);
    expect(find.text('发消息'), findsOneWidget);
  });
}
