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
  });
}
