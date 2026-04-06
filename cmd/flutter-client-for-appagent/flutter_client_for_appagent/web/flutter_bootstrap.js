{{flutter_js}}
{{flutter_build_config}}

(function () {
  const loader = document.getElementById('app-loader');
  const statusNode = document.getElementById('app-loader-status');
  const hintNode = document.getElementById('app-loader-hint');
  const footnoteNode = document.getElementById('app-loader-footnote');

  const setLoaderText = ({ status, hint, footnote }) => {
    if (statusNode && typeof status === 'string') {
      statusNode.textContent = status;
    }
    if (hintNode && typeof hint === 'string') {
      hintNode.textContent = hint;
    }
    if (footnoteNode && typeof footnote === 'string') {
      footnoteNode.textContent = footnote;
    }
  };

  const hideLoader = () => {
    if (!loader || loader.classList.contains('app-loader--hidden')) {
      return;
    }
    loader.classList.add('app-loader--hidden');
    window.setTimeout(() => {
      loader.remove();
    }, 320);
  };

  const failLoader = (message) => {
    setLoaderText({
      status: '页面加载失败',
      hint: message || '请刷新页面后重试。',
      footnote: '如果部署在静态服务器下，请检查 main.dart.js 和 canvaskit 资源是否能被正常访问。',
    });
  };

  window.setTimeout(() => {
    if (!loader || loader.classList.contains('app-loader--hidden')) {
      return;
    }
    setLoaderText({
      status: '资源加载中…',
      hint: '仍在下载 Flutter Web 运行时，首次访问这一步可能需要更久。',
      footnote: '如果你的网络较慢，canvaskit 相关 wasm 文件会明显拉长首屏时间。',
    });
  }, 4500);

  window.addEventListener('error', (event) => {
    const detail = event && event.message ? event.message : '';
    failLoader(detail);
  });

  window.addEventListener('unhandledrejection', (event) => {
    const reason = event && event.reason ? String(event.reason) : '';
    failLoader(reason);
  });

  _flutter.loader.load({
    serviceWorkerSettings: {
      serviceWorkerVersion: {{flutter_service_worker_version}}
    },
    onEntrypointLoaded: async function (engineInitializer) {
      setLoaderText({
        status: '正在启动应用…',
        hint: '资源已就绪，正在初始化 Flutter 引擎。',
      });

      const appRunner = await engineInitializer.initializeEngine();

      setLoaderText({
        status: '正在渲染界面…',
        hint: '应用即将完成首屏绘制。',
      });

      await appRunner.runApp();
      window.requestAnimationFrame(() => {
        window.requestAnimationFrame(hideLoader);
      });
    },
  });
})();
