import 'dart:io';
import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart';
import '../api/client.dart';
import '../models/file_item.dart';
import 'media_viewer_screen.dart';

class FileBrowserScreen extends StatefulWidget {
  const FileBrowserScreen({super.key});

  @override
  State<FileBrowserScreen> createState() => _FileBrowserScreenState();
}

class _FileBrowserScreenState extends State<FileBrowserScreen> {
  List<FileItem> _files = [];
  bool _loading = false;
  final List<_BreadcrumbEntry> _breadcrumbs = [_BreadcrumbEntry('root', 'Home')];

  String get _currentId => _breadcrumbs.last.id;

  @override
  void initState() {
    super.initState();
    _loadFiles();
  }

  Future<void> _loadFiles() async {
    setState(() => _loading = true);
    try {
      _files = await ApiClient().listFiles(parentId: _currentId);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Error: $e')));
    }
    if (mounted) setState(() => _loading = false);
  }

  void _openFolder(FileItem folder) {
    _breadcrumbs.add(_BreadcrumbEntry(folder.id, folder.name));
    _loadFiles();
  }

  void _navigateTo(int index) {
    _breadcrumbs.removeRange(index + 1, _breadcrumbs.length);
    _loadFiles();
  }

  void _openFile(FileItem file) {
    if (file.isViewable) {
      final viewableFiles = _files.where((f) => !f.isDir && f.isViewable).toList();
      final idx = viewableFiles.indexWhere((f) => f.id == file.id);
      Navigator.of(context).push(MaterialPageRoute(
        builder: (_) => MediaViewerScreen(files: viewableFiles, initialIndex: idx >= 0 ? idx : 0),
      ));
    }
  }

  Future<void> _uploadFile() async {
    final result = await FilePicker.platform.pickFiles(allowMultiple: true);
    if (result == null || result.files.isEmpty) return;

    for (final pf in result.files) {
      if (pf.path == null) continue;
      try {
        await ApiClient().uploadFile(_currentId, File(pf.path!), pf.name);
        if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Uploaded ${pf.name}')));
      } catch (e) {
        if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Failed: $e')));
      }
    }
    _loadFiles();
  }

  Future<void> _createFolder() async {
    final name = await showDialog<String>(
      context: context,
      builder: (ctx) {
        final controller = TextEditingController();
        return AlertDialog(
          title: const Text('New Folder'),
          content: TextField(controller: controller, autofocus: true, decoration: const InputDecoration(hintText: 'Folder name')),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
            TextButton(onPressed: () => Navigator.pop(ctx, controller.text), child: const Text('Create')),
          ],
        );
      },
    );
    if (name == null || name.trim().isEmpty) return;
    try {
      await ApiClient().createFolder(_currentId, name.trim());
      _loadFiles();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Error: $e')));
    }
  }

  Future<void> _deleteFile(FileItem file) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete'),
        content: Text('Delete "${file.name}"?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Delete', style: TextStyle(color: Colors.red))),
        ],
      ),
    );
    if (confirm != true) return;
    try {
      await ApiClient().deleteFile(file.id);
      _loadFiles();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Error: $e')));
    }
  }

  IconData _fileIcon(FileItem f) {
    if (f.isDir) return Icons.folder;
    if (f.isImage) return Icons.image;
    if (f.isVideo) return Icons.videocam;
    if (f.isAudio) return Icons.audiotrack;
    if (f.mimeType == 'application/pdf') return Icons.picture_as_pdf;
    return Icons.insert_drive_file;
  }

  Color _fileIconColor(FileItem f) {
    if (f.isDir) return const Color(0xFF4A8AFF);
    if (f.isImage) return Colors.green;
    if (f.isVideo) return Colors.orange;
    if (f.isAudio) return Colors.purple;
    return Colors.grey;
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // Breadcrumbs
        Container(
          height: 44,
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            itemCount: _breadcrumbs.length,
            separatorBuilder: (_, __) => const Padding(
              padding: EdgeInsets.symmetric(horizontal: 4),
              child: Icon(Icons.chevron_right, size: 18, color: Colors.grey),
            ),
            itemBuilder: (_, i) => Center(
              child: GestureDetector(
                onTap: i < _breadcrumbs.length - 1 ? () => _navigateTo(i) : null,
                child: Text(
                  _breadcrumbs[i].name,
                  style: TextStyle(
                    color: i == _breadcrumbs.length - 1 ? Colors.white : const Color(0xFF4A8AFF),
                    fontWeight: i == _breadcrumbs.length - 1 ? FontWeight.bold : FontWeight.normal,
                    fontSize: 14,
                  ),
                ),
              ),
            ),
          ),
        ),
        // Toolbar
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
          child: Row(
            children: [
              OutlinedButton.icon(onPressed: _uploadFile, icon: const Icon(Icons.upload_file, size: 18), label: const Text('Upload')),
              const SizedBox(width: 8),
              OutlinedButton.icon(onPressed: _createFolder, icon: const Icon(Icons.create_new_folder, size: 18), label: const Text('Folder')),
            ],
          ),
        ),
        // File list
        Expanded(
          child: RefreshIndicator(
            onRefresh: _loadFiles,
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _files.isEmpty
                    ? ListView(children: const [SizedBox(height: 100), Center(child: Text('Empty folder', style: TextStyle(color: Colors.grey)))])
                    : ListView.builder(
                        itemCount: _files.length,
                        itemBuilder: (_, i) {
                          final f = _files[i];
                          return ListTile(
                            leading: Icon(_fileIcon(f), color: _fileIconColor(f)),
                            title: Text(f.name, style: const TextStyle(color: Colors.white), overflow: TextOverflow.ellipsis),
                            subtitle: Text(
                              f.isDir ? 'Folder' : '${f.sizeFormatted} \u00B7 ${DateTime.tryParse(f.createdAt)?.toLocal().toString().split(' ').first ?? ''}',
                              style: const TextStyle(color: Colors.grey, fontSize: 12),
                            ),
                            trailing: IconButton(
                              icon: const Icon(Icons.more_vert, color: Colors.grey),
                              onPressed: () => _showFileMenu(f),
                            ),
                            onTap: () => f.isDir ? _openFolder(f) : _openFile(f),
                          );
                        },
                      ),
          ),
        ),
      ],
    );
  }

  void _showFileMenu(FileItem file) {
    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.delete, color: Colors.redAccent),
              title: const Text('Delete', style: TextStyle(color: Colors.redAccent)),
              onTap: () { Navigator.pop(ctx); _deleteFile(file); },
            ),
          ],
        ),
      ),
    );
  }
}

class _BreadcrumbEntry {
  final String id;
  final String name;
  _BreadcrumbEntry(this.id, this.name);
}
