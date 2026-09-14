# Flutter Chat App Frontend

Mobile app sử dụng Flutter với Clean Architecture.

## Tech Stack

- Flutter
- Dart
- State Management: Block
- Navigation: go_router
- HTTP Client: Dio
- Secure Storage: flutter_secure_storage

## Cấu trúc

```
lib/
  core/
    error/
    network/
    router/
    storage/
    utils/

  features/
    auth/
      presentation/
        pages/
        widgets/
        view_models/
      domain/
        entities/
        repositories/
        usecases/
      data/
        datasources/
        models/
        repositories/

    chat_list/
      presentation/
        pages/
        widgets/
        view_models/
      domain/
        entities/
        repositories/
        usecases/
      data/
        datasources/
        models/
        repositories/

    chat_detail/
      presentation/
        pages/
        widgets/
        view_models/
      domain/
        entities/
        repositories/
        usecases/
      data/
        datasources/
        models/
        repositories/

  main.dart
```

## Chạy app

```bash
flutter pub get
flutter run
```

## Build APK

```bash
flutter build apk
```
