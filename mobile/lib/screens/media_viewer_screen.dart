import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';
import '../api/client.dart';
import '../models/file_item.dart';

class MediaViewerScreen extends StatefulWidget {
  final List<FileItem> files;
  final int initialIndex;

  const MediaViewerScreen({super.key, required this.files, required this.initialIndex});

  @override
  State<MediaViewerScreen> createState() => _MediaViewerScreenState();
}

class _MediaViewerScreenState extends State<MediaViewerScreen> {
  late PageController _pageController;
  late int _currentIndex;

  @override
  void initState() {
    super.initState();
    _currentIndex = widget.initialIndex;
    _pageController = PageController(initialPage: _currentIndex);
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  FileItem get _currentFile => widget.files[_currentIndex];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      extendBodyBehindAppBar: true,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        elevation: 0,
        title: Text(_currentFile.name, style: const TextStyle(fontSize: 15)),
        actions: [
          IconButton(
            icon: const Icon(Icons.download),
            onPressed: () {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Open download URL in browser')),
              );
            },
          ),
        ],
      ),
      body: PageView.builder(
        controller: _pageController,
        itemCount: widget.files.length,
        onPageChanged: (i) => setState(() => _currentIndex = i),
        itemBuilder: (_, i) {
          final file = widget.files[i];
          if (file.isImage) return _ImageView(file: file);
          if (file.isVideo) return _VideoView(file: file);
          if (file.isAudio) return _AudioView(file: file);
          return Center(child: Text(file.name, style: const TextStyle(color: Colors.white)));
        },
      ),
      bottomNavigationBar: widget.files.length > 1
          ? Container(
              color: Colors.black,
              padding: const EdgeInsets.all(12),
              child: Text(
                '${_currentIndex + 1} / ${widget.files.length}',
                textAlign: TextAlign.center,
                style: const TextStyle(color: Colors.grey, fontSize: 13),
              ),
            )
          : null,
    );
  }
}

class _ImageView extends StatelessWidget {
  final FileItem file;
  const _ImageView({required this.file});

  @override
  Widget build(BuildContext context) {
    return InteractiveViewer(
      minScale: 0.5,
      maxScale: 4.0,
      child: Center(
        child: Image.network(
          ApiClient().viewUrl(file.id),
          fit: BoxFit.contain,
          loadingBuilder: (_, child, progress) {
            if (progress == null) return child;
            return Center(
              child: CircularProgressIndicator(
                value: progress.expectedTotalBytes != null
                    ? progress.cumulativeBytesLoaded / progress.expectedTotalBytes!
                    : null,
              ),
            );
          },
          errorBuilder: (_, __, ___) => const Icon(Icons.broken_image, color: Colors.grey, size: 64),
        ),
      ),
    );
  }
}

class _VideoView extends StatefulWidget {
  final FileItem file;
  const _VideoView({required this.file});

  @override
  State<_VideoView> createState() => _VideoViewState();
}

class _VideoViewState extends State<_VideoView> {
  late VideoPlayerController _controller;
  bool _initialized = false;

  @override
  void initState() {
    super.initState();
    _controller = VideoPlayerController.networkUrl(Uri.parse(ApiClient().viewUrl(widget.file.id)))
      ..initialize().then((_) {
        if (mounted) setState(() => _initialized = true);
        _controller.play();
      });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_initialized) {
      return const Center(child: CircularProgressIndicator());
    }
    return Center(
      child: AspectRatio(
        aspectRatio: _controller.value.aspectRatio,
        child: Stack(
          alignment: Alignment.bottomCenter,
          children: [
            VideoPlayer(_controller),
            _VideoControls(controller: _controller),
          ],
        ),
      ),
    );
  }
}

class _VideoControls extends StatelessWidget {
  final VideoPlayerController controller;
  const _VideoControls({required this.controller});

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder(
      valueListenable: controller,
      builder: (_, value, __) {
        return Container(
          color: Colors.black38,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Row(
            children: [
              IconButton(
                icon: Icon(value.isPlaying ? Icons.pause : Icons.play_arrow, color: Colors.white),
                onPressed: () => value.isPlaying ? controller.pause() : controller.play(),
              ),
              Expanded(
                child: VideoProgressIndicator(controller, allowScrubbing: true,
                  colors: const VideoProgressColors(playedColor: Color(0xFF4A8AFF)),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                '${_format(value.position)} / ${_format(value.duration)}',
                style: const TextStyle(color: Colors.white, fontSize: 12),
              ),
            ],
          ),
        );
      },
    );
  }

  String _format(Duration d) {
    final m = d.inMinutes.remainder(60).toString().padLeft(2, '0');
    final s = d.inSeconds.remainder(60).toString().padLeft(2, '0');
    if (d.inHours > 0) return '${d.inHours}:$m:$s';
    return '$m:$s';
  }
}

class _AudioView extends StatelessWidget {
  final FileItem file;
  const _AudioView({required this.file});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.audiotrack, color: Colors.white54, size: 80),
          const SizedBox(height: 16),
          Text(file.name, style: const TextStyle(color: Colors.white, fontSize: 16)),
          const SizedBox(height: 8),
          Text(file.sizeFormatted, style: const TextStyle(color: Colors.grey)),
        ],
      ),
    );
  }
}
