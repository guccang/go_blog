import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_client_for_appagent/main.dart';

void main() {
  testWidgets('renders app-agent client shell', (tester) async {
    await tester.pumpWidget(const AppAgentClientApp());

    expect(find.text('App Agent Client'), findsOneWidget);
    expect(find.text('Connect'), findsOneWidget);
    expect(find.text('Send'), findsOneWidget);
  });
}
