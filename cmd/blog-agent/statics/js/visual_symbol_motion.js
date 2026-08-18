(function () {
    'use strict';

    var profiles = {
        portal: ['pendulum', 'foliage-upper', 'foliage-lower', 'falling-leaves'],
        glass: ['glass-sheen'],
        water: ['water-ripple', 'water-ripple-late'],
        metal: ['metal-sheen']
    };

    function createLayer(symbol, effect) {
        var layer = document.createElement('span');
        layer.className = 'symbol-motion__layer symbol-motion__layer--' + effect;
        layer.setAttribute('aria-hidden', 'true');
        if (effect.indexOf('foliage-') === 0) {
            layer.dataset.symbol = symbol.dataset.symbol;
        }
        return layer;
    }

    function createFallingLeaves(stack) {
        for (var index = 0; index < 3; index += 1) {
            var leaf = document.createElement('i');
            leaf.className = 'symbol-motion__leaf symbol-motion__leaf--' + (index + 1);
            leaf.setAttribute('aria-hidden', 'true');
            stack.appendChild(leaf);
        }
    }

    function prepareStack(symbol) {
        var stack = document.createElement('span');
        stack.className = 'symbol-motion__stack';
        symbol.parentNode.insertBefore(stack, symbol);
        stack.appendChild(symbol);
        symbol.classList.add('symbol-motion__art');
        return stack;
    }

    function enhanceSymbol(symbol) {
        if (symbol.dataset.symbolMotionReady === 'true') return;

        var profileName = symbol.dataset.symbolMotion;
        var effects = profiles[profileName];
        if (!effects) return;

        var host = symbol.closest('.site-page-hero__exhibit') || symbol.parentElement;
        var stack = prepareStack(symbol);
        host.classList.add('symbol-motion-host', 'symbol-motion-host--' + profileName);
        symbol.dataset.symbolMotionReady = 'true';

        effects.forEach(function (effect) {
            if (effect === 'pendulum') {
                host.classList.add('symbol-motion-host--pendulum');
                return;
            }
            if (effect === 'falling-leaves') {
                createFallingLeaves(stack);
                return;
            }
            stack.appendChild(createLayer(symbol, effect));
        });

        return host;
    }

    function observeMotion(hosts) {
        if (!('IntersectionObserver' in window)) return;
        var observer = new IntersectionObserver(function (entries) {
            entries.forEach(function (entry) {
                entry.target.classList.toggle('is-motion-paused', !entry.isIntersecting);
            });
        }, { rootMargin: '80px' });
        hosts.forEach(function (host) { observer.observe(host); });
    }

    function init() {
        var hosts = [];
        document.querySelectorAll('[data-symbol-motion]').forEach(function (symbol) {
            var host = enhanceSymbol(symbol);
            if (host && hosts.indexOf(host) === -1) hosts.push(host);
        });
        observeMotion(hosts);
    }

    window.GuCcangSymbolMotion = {
        enhance: enhanceSymbol,
        profiles: profiles
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init, { once: true });
    } else {
        init();
    }
})();
