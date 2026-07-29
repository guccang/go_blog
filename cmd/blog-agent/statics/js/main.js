(function () {
    'use strict';

    var form = document.getElementById('workspaceQueryForm');
    var input = document.getElementById('workspaceQuery');
    var askButton = document.getElementById('askPIButton');
    var hint = document.getElementById('queryHint');
    var moreButton = document.getElementById('moreMenuButton');
    var moreMenu = document.getElementById('moreMenu');
    var quoteText = document.getElementById('dailyQuote');
    var quoteSource = document.getElementById('dailyQuoteSource');

    var quotes = [
        ['把今天能完成的一件小事，做到确实完成。', '给正在行动的自己'],
        ['生活不是等待风暴过去，而是学会在雨中跳舞。', '维维安·格林'],
        ['慢一点没关系，方向对了就仍在抵达。', '给长期主义者'],
        ['你不必很厉害才开始，但要开始才会很厉害。', '给今天的第一步'],
        ['真正重要的事，往往安静地发生在重复里。', '给持续练习的人']
    ];
    var dailyQuote = quotes[(new Date().getDate() - 1) % quotes.length];
    if (quoteText && quoteSource) {
        quoteText.textContent = dailyQuote[0];
        quoteSource.textContent = '— ' + dailyQuote[1];
    }

    document.querySelectorAll('.recent-card-image').forEach(function (image) {
        image.addEventListener('error', function () {
            var card = image.closest('.recent-card');
            image.remove();
            if (card) card.classList.remove('has-media');
        });
    });

    function queryValue() {
        return (input.value || '').trim();
    }

    function openQuery(path, parameter) {
        var query = queryValue();
        if (!query) {
            hint.textContent = '先输入想找的内容或问题。';
            input.focus();
            return;
        }
        window.location.assign(path + '?' + parameter + '=' + encodeURIComponent(query));
    }

    form.addEventListener('submit', function (event) {
        event.preventDefault();
        openQuery('/search', 'match');
    });

    askButton.addEventListener('click', function () {
        openQuery('/ask', 'q');
    });

    function setMoreOpen(open) {
        moreMenu.hidden = !open;
        moreButton.setAttribute('aria-expanded', String(open));
    }

    moreButton.addEventListener('click', function () {
        setMoreOpen(moreButton.getAttribute('aria-expanded') !== 'true');
    });
    document.addEventListener('click', function (event) {
        if (!event.target.closest('.more-menu')) setMoreOpen(false);
    });
    document.addEventListener('keydown', function (event) {
        if (event.key === 'Escape') {
            setMoreOpen(false);
            moreButton.focus();
        }
    });
})();
