class FileItem {
  final String id;
  final String? parentId;
  final String name;
  final bool isDir;
  final int sizeBytes;
  final String? mimeType;
  final String createdAt;
  final String updatedAt;

  FileItem({
    required this.id,
    this.parentId,
    required this.name,
    required this.isDir,
    required this.sizeBytes,
    this.mimeType,
    required this.createdAt,
    required this.updatedAt,
  });

  factory FileItem.fromJson(Map<String, dynamic> json) => FileItem(
        id: json['id'],
        parentId: json['parent_id'],
        name: json['name'],
        isDir: json['is_dir'] ?? false,
        sizeBytes: json['size_bytes'] ?? 0,
        mimeType: json['mime_type'],
        createdAt: json['created_at'] ?? '',
        updatedAt: json['updated_at'] ?? '',
      );

  bool get isImage => mimeType?.startsWith('image/') ?? false;
  bool get isVideo => mimeType?.startsWith('video/') ?? false;
  bool get isAudio => mimeType?.startsWith('audio/') ?? false;
  bool get isMedia => isImage || isVideo || isAudio;
  bool get isViewable => isMedia || mimeType == 'application/pdf';

  String get sizeFormatted {
    if (sizeBytes == 0) return '0 B';
    const suffixes = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = 0;
    var size = sizeBytes.toDouble();
    while (size >= 1024 && i < suffixes.length - 1) {
      size /= 1024;
      i++;
    }
    return '${size.toStringAsFixed(1)} ${suffixes[i]}';
  }
}

class PhotoItem extends FileItem {
  final String? takenAt;
  final String? cameraMake;
  final String? cameraModel;
  final int? width;
  final int? height;

  PhotoItem({
    required super.id,
    super.parentId,
    required super.name,
    required super.isDir,
    required super.sizeBytes,
    super.mimeType,
    required super.createdAt,
    required super.updatedAt,
    this.takenAt,
    this.cameraMake,
    this.cameraModel,
    this.width,
    this.height,
  });

  factory PhotoItem.fromJson(Map<String, dynamic> json) => PhotoItem(
        id: json['id'],
        parentId: json['parent_id'],
        name: json['name'],
        isDir: json['is_dir'] ?? false,
        sizeBytes: json['size_bytes'] ?? 0,
        mimeType: json['mime_type'],
        createdAt: json['created_at'] ?? '',
        updatedAt: json['updated_at'] ?? '',
        takenAt: json['taken_at'],
        cameraMake: json['camera_make'],
        cameraModel: json['camera_model'],
        width: json['width'],
        height: json['height'],
      );
}
