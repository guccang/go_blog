import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_client_for_appagent/main.dart';

void main() {
  group('AppAgentClient.downloadAttachmentToFile', () {
    Future<AppAgentClient> createClient(HttpServer server) async {
      final host = server.address.address;
      return AppAgentClient(
        baseUrl: 'http://$host:${server.port}',
        userId: 'ztt',
        password: '',
        receiveToken: '',
        sessionToken: 'token',
      );
    }

    test('resumes download after stream closes mid-transfer', () async {
      final payload = List<int>.generate(96 * 1024, (index) => index % 251);
      final midpoint = payload.length ~/ 2;
      final requestedRanges = <String?>[];
      var requestCount = 0;

      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        requestCount++;
        requestedRanges.add(request.headers.value(HttpHeaders.rangeHeader));
        if (request.uri.path != '/api/app/attachments/test-apk') {
          request.response.statusCode = HttpStatus.notFound;
          await request.response.close();
          return;
        }

        if (requestCount == 1) {
          request.response.statusCode = HttpStatus.ok;
          request.response.headers.contentLength = payload.length;
          final socket = await request.response.detachSocket(
            writeHeaders: true,
          );
          socket.add(payload.sublist(0, midpoint));
          await socket.flush();
          await socket.close();
          return;
        }

        final rangeHeader =
            request.headers.value(HttpHeaders.rangeHeader) ?? '';
        final match = RegExp(r'bytes=(\d+)-').firstMatch(rangeHeader);
        final start = int.parse(match!.group(1)!);
        request.response.statusCode = HttpStatus.partialContent;
        request.response.headers.contentLength = payload.length - start;
        request.response.headers.set(
          HttpHeaders.contentRangeHeader,
          'bytes $start-${payload.length - 1}/${payload.length}',
        );
        request.response.add(payload.sublist(start));
        await request.response.close();
      });

      final tempDir = await Directory.systemTemp.createTemp(
        'download_attachment_resume_',
      );
      addTearDown(() => tempDir.delete(recursive: true));
      final destinationPath = '${tempDir.path}/app-release.apk';
      final resumedEvents = <bool>[];

      final client = await createClient(server);
      await client.downloadAttachmentToFile(
        'test-apk',
        destinationPath: destinationPath,
        onProgress: (receivedBytes, totalBytes, resumed) {
          resumedEvents.add(resumed);
        },
      );

      expect(await File(destinationPath).readAsBytes(), payload);
      expect(await File('$destinationPath.part').exists(), isFalse);
      expect(requestCount, 2);
      expect(requestedRanges, <String?>[null, 'bytes=$midpoint-']);
      expect(resumedEvents, contains(true));
    });

    test('restarts from zero when server ignores range requests', () async {
      final payload = List<int>.generate(
        32 * 1024,
        (index) => (index * 7) % 253,
      );
      final existingBytes = payload.length ~/ 3;
      final requestedRanges = <String?>[];
      var requestCount = 0;

      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        requestCount++;
        requestedRanges.add(request.headers.value(HttpHeaders.rangeHeader));
        request.response.statusCode = HttpStatus.ok;
        request.response.headers.contentLength = payload.length;
        request.response.add(payload);
        await request.response.close();
      });

      final tempDir = await Directory.systemTemp.createTemp(
        'download_attachment_restart_',
      );
      addTearDown(() => tempDir.delete(recursive: true));
      final destinationPath = '${tempDir.path}/attachment.bin';
      await File(
        '$destinationPath.part',
      ).writeAsBytes(payload.sublist(0, existingBytes), flush: true);

      final client = await createClient(server);
      await client.downloadAttachmentToFile(
        'test-file',
        destinationPath: destinationPath,
      );

      expect(await File(destinationPath).readAsBytes(), payload);
      expect(await File('$destinationPath.part').exists(), isFalse);
      expect(requestCount, 2);
      expect(requestedRanges, <String?>['bytes=$existingBytes-', null]);
    });

    test('does not retry non-recoverable http failures', () async {
      var requestCount = 0;

      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        requestCount++;
        request.response.statusCode = HttpStatus.notFound;
        request.response.write('missing');
        await request.response.close();
      });

      final tempDir = await Directory.systemTemp.createTemp(
        'download_attachment_404_',
      );
      addTearDown(() => tempDir.delete(recursive: true));
      final destinationPath = '${tempDir.path}/missing.bin';

      final client = await createClient(server);
      await expectLater(
        client.downloadAttachmentToFile(
          'missing',
          destinationPath: destinationPath,
        ),
        throwsA(isA<HttpException>()),
      );

      expect(requestCount, 1);
      expect(await File(destinationPath).exists(), isFalse);
      expect(await File('$destinationPath.part').exists(), isFalse);
    });

    test('downloads via app-agent redirect to direct download url', () async {
      final payload = List<int>.generate(24 * 1024, (index) => index % 251);
      var appRequestCount = 0;
      var directRequestCount = 0;

      final directServer = await HttpServer.bind(
        InternetAddress.loopbackIPv4,
        0,
      );
      addTearDown(() => directServer.close(force: true));
      directServer.listen((request) async {
        directRequestCount++;
        request.response.statusCode = HttpStatus.ok;
        request.response.headers.contentLength = payload.length;
        request.response.add(payload);
        await request.response.close();
      });

      final appServer = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => appServer.close(force: true));
      appServer.listen((request) async {
        appRequestCount++;
        final host = directServer.address.address;
        final url = 'http://$host:${directServer.port}/object.bin';
        request.response.statusCode = HttpStatus.found;
        request.response.headers.set(HttpHeaders.locationHeader, url);
        await request.response.close();
      });

      final tempDir = await Directory.systemTemp.createTemp(
        'download_attachment_obs_',
      );
      addTearDown(() => tempDir.delete(recursive: true));
      final destinationPath = '${tempDir.path}/attachment.bin';

      final client = await createClient(appServer);
      await client.downloadAttachmentToFile(
        'test-file',
        destinationPath: destinationPath,
      );

      expect(await File(destinationPath).readAsBytes(), payload);
      expect(directRequestCount, 1);
      expect(appRequestCount, 1);
    });

    test('downloads directly from app-agent when no redirect is returned', () async {
      final payload = List<int>.generate(18 * 1024, (index) => index % 233);
      var appRequestCount = 0;

      final appServer = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => appServer.close(force: true));
      appServer.listen((request) async {
        appRequestCount++;
        request.response.statusCode = HttpStatus.ok;
        request.response.headers.contentLength = payload.length;
        request.response.add(payload);
        await request.response.close();
      });

      final tempDir = await Directory.systemTemp.createTemp(
        'download_attachment_obs_fallback_',
      );
      addTearDown(() => tempDir.delete(recursive: true));
      final destinationPath = '${tempDir.path}/attachment.bin';

      final client = await createClient(appServer);
      await client.downloadAttachmentToFile(
        'test-file',
        destinationPath: destinationPath,
      );

      expect(await File(destinationPath).readAsBytes(), payload);
      expect(appRequestCount, 1);
    });
  });
}
