import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import '../models/file_item.dart';

class ApiClient {
  String _baseUrl = '';
  String? _accessToken;
  String? _refreshToken;
  VoidCallback? onAuthFailure;

  static final ApiClient _instance = ApiClient._();
  factory ApiClient() => _instance;
  ApiClient._();

  String get baseUrl => _baseUrl;
  bool get isConfigured => _baseUrl.isNotEmpty;
  bool get isLoggedIn => _accessToken != null;

  Future<void> init() async {
    final prefs = await SharedPreferences.getInstance();
    _baseUrl = prefs.getString('server_url') ?? '';
    _accessToken = prefs.getString('access_token');
    _refreshToken = prefs.getString('refresh_token');
  }

  Future<void> setServer(String url) async {
    _baseUrl = url.endsWith('/') ? url.substring(0, url.length - 1) : url;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('server_url', _baseUrl);
  }

  Future<void> _saveTokens(String access, String refresh) async {
    _accessToken = access;
    _refreshToken = refresh;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('access_token', access);
    await prefs.setString('refresh_token', refresh);
  }

  Future<void> clearTokens() async {
    _accessToken = null;
    _refreshToken = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('access_token');
    await prefs.remove('refresh_token');
    await prefs.remove('user');
  }

  String viewUrl(String fileId) =>
      '$_baseUrl/api/v1/files/$fileId/view?token=${Uri.encodeComponent(_accessToken ?? '')}';

  String downloadUrl(String fileId) =>
      '$_baseUrl/api/v1/files/$fileId/download?token=${Uri.encodeComponent(_accessToken ?? '')}';

  String thumbUrl(String fileId, {String size = 'sm'}) =>
      '$_baseUrl/api/v1/files/$fileId/thumb?size=$size&token=${Uri.encodeComponent(_accessToken ?? '')}';

  // --- Auth ---

  Future<Map<String, dynamic>> login(String username, String password) async {
    final resp = await http.post(
      Uri.parse('$_baseUrl/api/v1/auth/login'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'username': username, 'password': password}),
    );
    final data = jsonDecode(resp.body);
    if (resp.statusCode != 200) throw Exception(data['error'] ?? 'Login failed');
    await _saveTokens(data['access_token'], data['refresh_token']);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('user', jsonEncode(data['user']));
    return data['user'];
  }

  Future<void> logout() async {
    try {
      await _authRequest('POST', '/api/v1/auth/logout',
          body: {'refresh_token': _refreshToken});
    } catch (_) {}
    await clearTokens();
  }

  Future<bool> _tryRefresh() async {
    if (_refreshToken == null) return false;
    try {
      final resp = await http.post(
        Uri.parse('$_baseUrl/api/v1/auth/refresh'),
        headers: {'Content-Type': 'application/json'},
        body: jsonEncode({'refresh_token': _refreshToken}),
      );
      if (resp.statusCode != 200) return false;
      final data = jsonDecode(resp.body);
      await _saveTokens(data['access_token'], data['refresh_token']);
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<http.Response> _authRequest(String method, String path,
      {Map<String, dynamic>? body, Map<String, String>? extraHeaders}) async {
    var headers = <String, String>{
      if (_accessToken != null) 'Authorization': 'Bearer $_accessToken',
      ...?extraHeaders,
    };
    if (body != null) headers['Content-Type'] = 'application/json';

    var resp = await _rawRequest(method, path, headers: headers, body: body);

    if (resp.statusCode == 401 && _refreshToken != null) {
      if (await _tryRefresh()) {
        headers['Authorization'] = 'Bearer $_accessToken';
        resp = await _rawRequest(method, path, headers: headers, body: body);
      } else {
        onAuthFailure?.call();
        throw Exception('Session expired');
      }
    }
    return resp;
  }

  Future<http.Response> _rawRequest(String method, String path,
      {Map<String, String>? headers, Map<String, dynamic>? body}) async {
    final uri = Uri.parse('$_baseUrl$path');
    switch (method) {
      case 'GET':
        return http.get(uri, headers: headers);
      case 'POST':
        return http.post(uri, headers: headers, body: body != null ? jsonEncode(body) : null);
      case 'PUT':
        return http.put(uri, headers: headers, body: body != null ? jsonEncode(body) : null);
      case 'DELETE':
        return http.delete(uri, headers: headers);
      default:
        throw Exception('Unknown method: $method');
    }
  }

  // --- Files ---

  Future<List<FileItem>> listFiles({String parentId = 'root'}) async {
    final path = parentId == 'root' ? '/api/v1/files' : '/api/v1/files/$parentId/children';
    final resp = await _authRequest('GET', path);
    if (resp.statusCode != 200) throw Exception('Failed to load files');
    final list = jsonDecode(resp.body) as List;
    return list.map((j) => FileItem.fromJson(j)).toList();
  }

  Future<FileItem> createFolder(String parentId, String name) async {
    final resp = await _authRequest('POST', '/api/v1/files',
        body: {'parent_id': parentId, 'name': name});
    final data = jsonDecode(resp.body);
    if (resp.statusCode != 201) throw Exception(data['error'] ?? 'Failed');
    return FileItem.fromJson(data);
  }

  Future<FileItem> uploadFile(String parentId, File file, String filename) async {
    final uri = Uri.parse('$_baseUrl/api/v1/files');
    final req = http.MultipartRequest('POST', uri);
    if (_accessToken != null) req.headers['Authorization'] = 'Bearer $_accessToken';
    req.fields['parent_id'] = parentId;
    req.files.add(await http.MultipartFile.fromPath('file', file.path, filename: filename));
    final streamed = await req.send();
    final resp = await http.Response.fromStream(streamed);
    final data = jsonDecode(resp.body);
    if (resp.statusCode != 201) throw Exception(data['error'] ?? 'Upload failed');
    return FileItem.fromJson(data);
  }

  Future<void> deleteFile(String id) async {
    final resp = await _authRequest('DELETE', '/api/v1/files/$id');
    if (resp.statusCode != 200) throw Exception('Delete failed');
  }

  // --- Photos ---

  Future<Map<String, dynamic>> listPhotos({int limit = 50, int offset = 0}) async {
    final resp = await _authRequest('GET', '/api/v1/photos?limit=$limit&offset=$offset');
    return jsonDecode(resp.body);
  }

  // --- Ping ---

  Future<Map<String, dynamic>> ping() async {
    final resp = await http.get(Uri.parse('$_baseUrl/api/v1/ping'));
    if (resp.statusCode != 200) throw Exception('Server unreachable');
    return jsonDecode(resp.body);
  }
}

typedef VoidCallback = void Function();
