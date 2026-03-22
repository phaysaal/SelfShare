import 'package:flutter_test/flutter_test.dart';
import 'package:selfshare/main.dart';

void main() {
  testWidgets('App renders', (WidgetTester tester) async {
    await tester.pumpWidget(const SelfShareApp());
    expect(find.text('SelfShare'), findsOneWidget);
  });
}
