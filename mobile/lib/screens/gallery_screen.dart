import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import '../api/client.dart';
import '../models/file_item.dart';
import 'media_viewer_screen.dart';

class GalleryScreen extends StatefulWidget {
  const GalleryScreen({super.key});

  @override
  State<GalleryScreen> createState() => _GalleryScreenState();
}

class _GalleryScreenState extends State<GalleryScreen> {
  List<PhotoItem> _photos = [];
  int _total = 0;
  bool _loading = false;
  final _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    _load(0);
    _scrollController.addListener(() {
      if (_scrollController.position.pixels >= _scrollController.position.maxScrollExtent - 200) {
        if (!_loading && _photos.length < _total) _load(_photos.length);
      }
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  Future<void> _load(int offset) async {
    if (_loading) return;
    setState(() => _loading = true);
    try {
      final data = await ApiClient().listPhotos(limit: 60, offset: offset);
      final list = (data['photos'] as List?)?.map((j) => PhotoItem.fromJson(j)).toList() ?? [];
      setState(() {
        if (offset == 0) {
          _photos = list;
        } else {
          _photos.addAll(list);
        }
        _total = data['total'] ?? 0;
      });
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Error: $e')));
    }
    if (mounted) setState(() => _loading = false);
  }

  void _openPhoto(int index) {
    final files = _photos.map((p) => p as FileItem).toList();
    Navigator.of(context).push(MaterialPageRoute(
      builder: (_) => MediaViewerScreen(files: files, initialIndex: index),
    ));
  }

  @override
  Widget build(BuildContext context) {
    if (_photos.isEmpty && _loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_photos.isEmpty) {
      return const Center(child: Text('No photos yet', style: TextStyle(color: Colors.grey)));
    }

    return RefreshIndicator(
      onRefresh: () => _load(0),
      child: GridView.builder(
        controller: _scrollController,
        padding: const EdgeInsets.all(4),
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 3,
          crossAxisSpacing: 2,
          mainAxisSpacing: 2,
        ),
        itemCount: _photos.length + (_photos.length < _total ? 1 : 0),
        itemBuilder: (_, i) {
          if (i >= _photos.length) {
            return const Center(child: Padding(padding: EdgeInsets.all(16), child: CircularProgressIndicator(strokeWidth: 2)));
          }
          final photo = _photos[i];
          return GestureDetector(
            onTap: () => _openPhoto(i),
            child: Stack(
              fit: StackFit.expand,
              children: [
                CachedNetworkImage(
                  imageUrl: ApiClient().thumbUrl(photo.id, size: 'sm'),
                  fit: BoxFit.cover,
                  placeholder: (_, __) => Container(color: const Color(0xFF1A1A1A)),
                  errorWidget: (_, __, ___) => Container(
                    color: const Color(0xFF1A1A1A),
                    child: const Icon(Icons.broken_image, color: Colors.grey),
                  ),
                ),
                if (photo.isVideo)
                  Positioned(
                    bottom: 4, right: 4,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                      decoration: BoxDecoration(color: Colors.black54, borderRadius: BorderRadius.circular(4)),
                      child: const Icon(Icons.play_arrow, color: Colors.white, size: 16),
                    ),
                  ),
              ],
            ),
          );
        },
      ),
    );
  }
}
