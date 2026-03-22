import 'package:flutter/material.dart';
import 'api/client.dart';
import 'screens/connect_screen.dart';
import 'screens/home_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await ApiClient().init();
  runApp(const SelfShareApp());
}

class SelfShareApp extends StatelessWidget {
  const SelfShareApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'SelfShare',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        brightness: Brightness.dark,
        scaffoldBackgroundColor: const Color(0xFF0A0A0A),
        colorSchemeSeed: const Color(0xFF4A8AFF),
        useMaterial3: true,
        appBarTheme: const AppBarTheme(
          backgroundColor: Color(0xFF111111),
          foregroundColor: Colors.white,
        ),
      ),
      home: ApiClient().isLoggedIn ? const HomeScreen() : const ConnectScreen(),
    );
  }
}
