import 'dart:convert';
import 'dart:io';

import 'package:flutter_client_for_appagent/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('AppAgentClient auth flow', () {
    test('login parses access and refresh tokens', () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        expect(request.uri.path, '/api/app/login');
        expect(request.headers.value('X-App-Agent-Token'), 'receive-token');

        final payload =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        expect(payload['user_id'], 'demo-user');
        expect(payload['password'], 'demo-password');

        request.response.statusCode = HttpStatus.ok;
        request.response.headers.contentType = ContentType.json;
        request.response.write(
          jsonEncode(<String, dynamic>{
            'success': true,
            'access_token': 'access-1',
            'refresh_token': 'refresh-1',
            'user_id': 'demo-user',
            'expires_at': 1,
            'obs_agent_base_url': 'http://obs.local',
          }),
        );
        await request.response.close();
      });

      final host = server.address.address;
      final client = AppAgentClient(
        baseUrl: 'http://$host:${server.port}',
        userId: 'demo-user',
        password: 'demo-password',
        receiveToken: 'receive-token',
        sessionToken: '',
      );

      final session = await client.login();
      expect(session.userId, 'demo-user');
      expect(session.accessToken, 'access-1');
      expect(session.refreshToken, 'refresh-1');
      expect(session.expiresAtMs, 1);
      expect(session.obsAgentBaseUrl, 'http://obs.local');
    });

    test('refresh rotates token and logout sends refresh token', () async {
      final observedPaths = <String>[];
      final observedRefreshTokens = <String>[];

      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        observedPaths.add(request.uri.path);
        final payload =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;

        switch (request.uri.path) {
          case '/api/app/refresh':
            observedRefreshTokens.add(
              (payload['refresh_token'] ?? '').toString(),
            );
            request.response.statusCode = HttpStatus.ok;
            request.response.headers.contentType = ContentType.json;
            request.response.write(
              jsonEncode(<String, dynamic>{
                'success': true,
                'session_token': 'access-2',
                'refresh_token': 'refresh-2',
                'user_id': 'demo-user',
                'expires_in': 60,
              }),
            );
            break;
          case '/api/app/logout':
            observedRefreshTokens.add(
              (payload['refresh_token'] ?? '').toString(),
            );
            expect(request.headers.value('X-App-Agent-Session'), 'access-1');
            request.response.statusCode = HttpStatus.ok;
            request.response.headers.contentType = ContentType.json;
            request.response.write('{"success":true}');
            break;
          default:
            request.response.statusCode = HttpStatus.notFound;
            request.response.write('missing');
        }
        await request.response.close();
      });

      final host = server.address.address;
      final client = AppAgentClient(
        baseUrl: 'http://$host:${server.port}',
        userId: 'demo-user',
        password: '',
        receiveToken: 'receive-token',
        sessionToken: 'access-1',
      );

      final refreshed = await client.refreshSession('refresh-1');
      expect(refreshed.accessToken, 'access-2');
      expect(refreshed.refreshToken, 'refresh-2');
      expect(refreshed.expiresAtMs, greaterThan(0));

      await client.logout(refreshToken: 'refresh-2');

      expect(observedPaths, <String>['/api/app/refresh', '/api/app/logout']);
      expect(observedRefreshTokens, <String>['refresh-1', 'refresh-2']);
    });

    test('log APIs fetch configured source and latest 100 lines', () async {
      final observed = <Uri>[];
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        observed.add(request.uri);
        expect(request.headers.value('X-App-Agent-Session'), 'access-1');
        request.response.statusCode = HttpStatus.ok;
        request.response.headers.contentType = ContentType.json;
        switch (request.uri.path) {
          case '/api/app/logs/sources':
            request.response.write(
              jsonEncode(<String, dynamic>{
                'success': true,
                'sources': <Map<String, dynamic>>[
                  <String, dynamic>{
                    'name': 'app-agent',
                    'path': '/logs/app',
                    'description': 'app logs',
                  },
                ],
              }),
            );
            break;
          case '/api/app/logs/content':
            expect(request.uri.queryParameters['source'], 'app-agent');
            expect(request.uri.queryParameters['lines'], '100');
            request.response.write(
              jsonEncode(<String, dynamic>{
                'success': true,
                'source': <String, dynamic>{'name': 'app-agent'},
                'file': 'app.log',
                'content': 'line 1\nline 2',
                'matched_lines': 2,
                'truncated': false,
              }),
            );
            break;
          default:
            request.response.statusCode = HttpStatus.notFound;
            request.response.write('missing');
        }
        await request.response.close();
      });

      final host = server.address.address;
      final client = AppAgentClient(
        baseUrl: 'http://$host:${server.port}',
        userId: 'demo-user',
        password: '',
        receiveToken: 'receive-token',
        sessionToken: 'access-1',
      );

      final sources = await client.listLogSources();
      expect(sources.single.name, 'app-agent');

      final content = await client.readLogContent(
        source: sources.single.name,
        lines: 100,
      );
      expect(content.file, 'app.log');
      expect(content.content, contains('line 2'));
      expect(observed.map((uri) => uri.path).toList(), <String>[
        '/api/app/logs/sources',
        '/api/app/logs/content',
      ]);
    });

    test('butler APIs do not duplicate api app base path', () async {
      final observed = <Uri>[];
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        observed.add(request.uri);
        request.response.headers.contentType = ContentType.json;
        switch (request.uri.path) {
          case '/api/app/butler/feedback':
            request.response.statusCode = HttpStatus.ok;
            request.response.write(
              jsonEncode(<String, dynamic>{
                'success': true,
                'affinity': <String, dynamic>{'score': 53},
              }),
            );
            break;
          case '/api/app/butler/affinity':
            request.response.statusCode = HttpStatus.ok;
            request.response.write(
              jsonEncode(<String, dynamic>{
                'success': true,
                'affinity': <String, dynamic>{'score': 53},
              }),
            );
            break;
          default:
            request.response.statusCode = HttpStatus.notFound;
            request.response.write('missing');
        }
        await request.response.close();
      });

      final host = server.address.address;
      final client = AppAgentClient(
        baseUrl: 'http://$host:${server.port}/api/app/',
        userId: 'demo-user',
        password: '',
        receiveToken: 'receive-token',
        sessionToken: 'access-1',
      );

      await client.callButlerFeedback('helpful');
      await client.fetchButlerAffinity();

      expect(observed.map((uri) => uri.path).toList(), <String>[
        '/api/app/butler/feedback',
        '/api/app/butler/affinity',
      ]);
      expect(
        observed.last.queryParameters['session_token'],
        'access-1',
      );
    });

    test('app APIs do not duplicate api app base path', () async {
      final observed = <Uri>[];
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        observed.add(request.uri);
        request.response.headers.contentType = ContentType.json;
        switch (request.uri.path) {
          case '/api/app/login':
            request.response.statusCode = HttpStatus.ok;
            request.response.write(
              jsonEncode(<String, dynamic>{
                'success': true,
                'access_token': 'access-1',
                'refresh_token': 'refresh-1',
                'user_id': 'demo-user',
                'expires_at': 1,
              }),
            );
            break;
          case '/api/app/logs/sources':
            request.response.statusCode = HttpStatus.ok;
            request.response.write(
              jsonEncode(<String, dynamic>{
                'success': true,
                'sources': <Map<String, dynamic>>[
                  <String, dynamic>{'name': 'app-agent'},
                ],
              }),
            );
            break;
          case '/api/app/attachments/test-file':
            expect(request.uri.queryParameters['user_id'], 'demo-user');
            request.response.statusCode = HttpStatus.ok;
            request.response.headers.contentType = ContentType.binary;
            request.response.add(<int>[1, 2, 3]);
            break;
          default:
            request.response.statusCode = HttpStatus.notFound;
            request.response.write('missing');
        }
        await request.response.close();
      });

      final host = server.address.address;
      final client = AppAgentClient(
        baseUrl: 'http://$host:${server.port}/api/app/',
        userId: 'demo-user',
        password: 'demo-password',
        receiveToken: 'receive-token',
        sessionToken: 'access-1',
      );

      await client.login();
      await client.listLogSources();
      final bytes = await client.downloadAttachment('test-file');

      expect(bytes, <int>[1, 2, 3]);
      expect(observed.map((uri) => uri.path).toList(), <String>[
        '/api/app/login',
        '/api/app/logs/sources',
        '/api/app/attachments/test-file',
      ]);
    });
  });
}
