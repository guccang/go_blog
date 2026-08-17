(() => {
    const frame = document.getElementById('viewer-frame');
    const sourceField = document.getElementById('media-source');
    const status = document.getElementById('viewer-status');
    const backButton = document.getElementById('viewer-back');
    if (!frame || !sourceField) return;

    const mode = document.body.dataset.viewMode || 'text';
    const source = sourceField.value;
    const frameStyle = `
        :root { color-scheme: light; }
        * { box-sizing: border-box; }
        html { background: #fffdf9; color: #243746; font-family: "Microsoft YaHei", "Noto Sans SC", sans-serif; }
        body { width: min(920px, calc(100% - 48px)); margin: 0 auto; padding: 48px 0 80px; font-size: 16px; line-height: 1.85; }
        h1, h2, h3, h4 { color: #1d3850; line-height: 1.3; letter-spacing: -.02em; }
        h1 { margin-top: 0; font-size: clamp(30px, 5vw, 48px); }
        h2 { margin-top: 2em; border-bottom: 1px solid #e2ddd4; padding-bottom: .35em; }
        a { color: #b75535; text-underline-offset: 3px; }
        img, video { max-width: 100%; height: auto; border-radius: 8px; }
        blockquote { margin: 1.5em 0; border-left: 4px solid #3f9c91; padding: .1em 1.2em; color: #596974; background: #f1f7f5; }
        pre { overflow: auto; border-radius: 8px; padding: 18px; background: #18374d; color: #eef5f4; font: 14px/1.75 "Cascadia Mono", "SFMono-Regular", Consolas, monospace; tab-size: 4; }
        code { border-radius: 4px; padding: .15em .35em; background: #edf1f2; font-family: "Cascadia Mono", "SFMono-Regular", Consolas, monospace; }
        pre code { padding: 0; background: transparent; }
        table { width: 100%; border-collapse: collapse; }
        th, td { border: 1px solid #d8dfe2; padding: 8px 11px; text-align: left; }
        th { background: #eef4f3; }
        .plain-text { width: 100%; margin: 0; overflow-wrap: anywhere; white-space: pre-wrap; color: #243746; background: transparent; font-size: 15px; line-height: 1.8; }
        @media (max-width: 640px) { body { width: min(100% - 28px, 920px); padding-top: 28px; } }
    `;
    const readerPolicy = "default-src 'none'; img-src data: blob: https: http:; media-src data: blob: https: http:; font-src data: https: http:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'";

    function plainTextHTML(value) {
        const pre = document.createElement('pre');
        pre.className = 'plain-text';
        pre.textContent = value;
        return pre.outerHTML;
    }

    function renderSource() {
        if (mode === 'html') {
            const renderURL = frame.dataset.renderUrl;
            if (renderURL) frame.src = renderURL;
            return;
        }
        let rendered = source;
        if (mode === 'markdown') {
            rendered = typeof marked === 'function' ? marked.parse(source) : plainTextHTML(source);
        } else if (mode === 'json') {
            try {
                rendered = plainTextHTML(JSON.stringify(JSON.parse(source), null, 2));
            } catch (_) {
                rendered = plainTextHTML(source);
            }
        } else {
            rendered = plainTextHTML(source);
        }

        const documentNode = new DOMParser().parseFromString(rendered, 'text/html');
        documentNode.querySelectorAll('script, form, object, embed, base').forEach((element) => element.remove());
        documentNode.querySelectorAll('meta[http-equiv="refresh" i]').forEach((element) => element.remove());
        documentNode.querySelectorAll('a[href]').forEach((link) => {
            link.target = '_blank';
            link.rel = 'noopener noreferrer';
        });

        const policy = documentNode.createElement('meta');
        policy.httpEquiv = 'Content-Security-Policy';
        policy.content = readerPolicy;
        documentNode.head.prepend(policy);
        if (!documentNode.querySelector('meta[name="viewport"]')) {
            const viewport = documentNode.createElement('meta');
            viewport.name = 'viewport';
            viewport.content = 'width=device-width, initial-scale=1';
            documentNode.head.prepend(viewport);
        }
        const style = documentNode.createElement('style');
        style.textContent = frameStyle;
        documentNode.head.append(style);
        frame.srcdoc = '<!doctype html>' + documentNode.documentElement.outerHTML;
    }

    frame.addEventListener('load', () => {
        status?.classList.add('is-ready');
    }, { once: true });
    backButton?.addEventListener('click', () => {
        if (window.history.length > 1) window.history.back();
        else window.location.href = '/main';
    });
    renderSource();
})();
