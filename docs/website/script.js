(function () {
  'use strict';

  const searchIndex = [];

  function initSearch() {
    const pages = [
      { title: 'Home', url: 'index.html', sections: ['Overview', 'Features', 'Quick Links', 'What\'s New', 'Comparison'] },
      { title: 'Getting Started', url: 'getting-started.html', sections: ['Installation', 'Configuration', 'Define Your First Model', 'CRUD Operations', 'Migrations', 'Relationships', 'Eager Loading', 'Scopes', 'Events', 'Serialization'] },
      { title: 'API Reference', url: 'api-reference.html', sections: ['Core Types', 'DB', 'Builder', 'ModelDB', 'RawQuery', 'Tx', 'Schema & Migrator', 'Blueprint', 'ConnectionManager', 'Paginator', 'EventDispatcher', 'ScopeRegistry', 'Dialect', 'Serialization', 'Casting', 'Macros', 'Sentinel Errors'] },
      { title: 'Examples', url: 'examples.html', sections: ['Basic CRUD', 'Complex Queries', 'Transactions', 'Migrations', 'Relationships', 'Eager Loading', 'Scopes', 'Raw SQL', 'Serialization', 'Events & Hooks', 'Macros', 'Real-World Blog API'] },
      { title: 'Architecture', url: 'architecture.html', sections: ['Package Structure', 'Dependency Graph', 'Data Flow', 'Design Decisions', 'Thread Safety', 'Security Considerations', 'Known Limitations'] },
      { title: 'FAQ', url: 'faq.html', sections: ['Databases', 'Comparison', 'Thread Safety', 'Connection Pooling', 'Migrations', 'Eager Loading', 'Transactions', 'Raw SQL', 'Soft Deletes', 'Relationships', 'Pagination', 'Multiple Connections', 'Events and Hooks', 'Casting', 'Master Replica Routing', 'Macros', 'Global Scopes', 'Testing', 'Production Readiness', 'Serialization'] },
      { title: '404 — Page Not Found', url: '404.html', sections: ['Page Not Found'] },
      { title: 'Comparison', url: 'comparison.html', sections: ['Feature Matrix', 'Performance', 'API Style Comparison', 'When to Choose'] },
      { title: 'Contributing', url: 'contributing.html', sections: ['Development Setup', 'Running Tests', 'Coding Standards', 'Project Structure', 'Pull Request Workflow', 'Commit Message Conventions', 'Issue Templates'] },
      { title: 'Changelog', url: 'changelog.html', sections: ['v2.0.0', 'v1.0.0', 'v0.1.0', 'v2.1.0 Planned', 'v3.0.0 Roadmap'] },
    ];
    pages.forEach(p => {
      p.sections.forEach(sec => {
        searchIndex.push({ title: p.title, url: p.url + '#' + slugify(sec), section: sec });
      });
    });
  }

  function slugify(s) {
    return s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
  }

  document.addEventListener('DOMContentLoaded', function () {
    initSearch();
    initThemeToggle();
    initMobileMenu();
    initActiveNav();
    initSearchUI();
    initApiToggles();
    initExampleTabs();
    initSmoothScroll();
    initCopyButtons();
    initKeyboardShortcuts();
    initFaqAccordion();
    initAnchorLinks();
  });

  function initThemeToggle() {
    const btn = document.getElementById('themeToggle');
    if (!btn) return;
    const stored = localStorage.getItem('theme');
    if (stored === 'dark') document.documentElement.setAttribute('data-theme', 'dark');
    btn.textContent = document.documentElement.getAttribute('data-theme') === 'dark' ? '☀️ Light' : '🌙 Dark';
    btn.addEventListener('click', function () {
      const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
      const next = isDark ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', next);
      localStorage.setItem('theme', next);
      btn.textContent = next === 'dark' ? '☀️ Light' : '🌙 Dark';
    });
  }

  function initMobileMenu() {
    const toggle = document.getElementById('menuToggle');
    const sidebar = document.getElementById('sidebar');
    if (!toggle || !sidebar) return;
    toggle.addEventListener('click', function () {
      sidebar.classList.toggle('open');
    });
    document.addEventListener('click', function (e) {
      if (!sidebar.contains(e.target) && !toggle.contains(e.target)) {
        sidebar.classList.remove('open');
      }
    });
  }

  function initActiveNav() {
    const current = window.location.pathname.split('/').pop() || 'index.html';
    document.querySelectorAll('.sidebar-nav a').forEach(function (a) {
      const href = a.getAttribute('href');
      if (href === current) a.classList.add('active');
      else a.classList.remove('active');
    });
  }

  function initSearchUI() {
    const input = document.getElementById('searchInput');
    const results = document.getElementById('searchResults');
    if (!input || !results) return;

    input.addEventListener('input', function () {
      const q = input.value.trim().toLowerCase();
      if (q.length < 2) { results.classList.remove('show'); return; }

      const hits = searchIndex.filter(function (item) {
        return item.title.toLowerCase().includes(q) || item.section.toLowerCase().includes(q);
      }).slice(0, 10);

      if (hits.length === 0) { results.classList.remove('show'); return; }

      results.innerHTML = hits.map(function (h) {
        const sec = highlight(h.section, q);
        const titl = highlight(h.title, q);
        return '<a href="' + h.url + '"><span class="match-highlight">' + titl + '</span> &mdash; <span class="context">' + sec + '</span></a>';
      }).join('');
      results.classList.add('show');
    });

    document.addEventListener('click', function (e) {
      if (!results.contains(e.target) && e.target !== input) {
        results.classList.remove('show');
      }
    });
  }

  function highlight(text, query) {
    const idx = text.toLowerCase().indexOf(query);
    if (idx === -1) return escapeHtml(text);
    return escapeHtml(text.slice(0, idx)) + '<strong>' + escapeHtml(text.slice(idx, idx + query.length)) + '</strong>' + escapeHtml(text.slice(idx + query.length));
  }

  function escapeHtml(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }

  function initApiToggles() {
    document.querySelectorAll('.api-entry-header').forEach(function (header) {
      header.addEventListener('click', function () {
        var body = header.nextElementSibling;
        if (body) body.classList.toggle('open');
      });
    });
  }

  function initExampleTabs() {
    document.querySelectorAll('.example-tabs').forEach(function (tabs) {
      tabs.querySelectorAll('.example-tab').forEach(function (tab) {
        tab.addEventListener('click', function () {
          var parent = tabs.parentElement;
          tabs.querySelectorAll('.example-tab').forEach(function (t) { t.classList.remove('active'); });
          tab.classList.add('active');
          parent.querySelectorAll('.example-panel').forEach(function (p) { p.classList.remove('active'); });
          var target = parent.querySelector('.example-panel[data-tab="' + tab.getAttribute('data-tab') + '"]');
          if (target) target.classList.add('active');
        });
      });
    });
  }

  function initSmoothScroll() {
    document.querySelectorAll('a[href^="#"]').forEach(function (a) {
      a.addEventListener('click', function (e) {
        var id = a.getAttribute('href').slice(1);
        var target = document.getElementById(id);
        if (target) {
          e.preventDefault();
          target.scrollIntoView({ behavior: 'smooth', block: 'start' });
          history.pushState(null, '', '#' + id);
        }
      });
    });
  }

  function initCopyButtons() {
    document.querySelectorAll('pre code').forEach(function (codeBlock) {
      var pre = codeBlock.parentElement;
      if (pre.querySelector('.copy-btn')) return;

      var btn = document.createElement('button');
      btn.className = 'copy-btn';
      btn.textContent = 'Copy';
      btn.setAttribute('aria-label', 'Copy code to clipboard');
      pre.style.position = 'relative';
      pre.insertBefore(btn, pre.firstChild);

      btn.addEventListener('click', function () {
        var text = codeBlock.textContent;
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text).then(function () {
            btn.textContent = 'Copied!';
            btn.classList.add('copied');
            setTimeout(function () {
              btn.textContent = 'Copy';
              btn.classList.remove('copied');
            }, 2000);
          }, function () {
            fallbackCopy(text, btn);
          });
        } else {
          fallbackCopy(text, btn);
        }
      });
    });
  }

  function fallbackCopy(text, btn) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand('copy');
      btn.textContent = 'Copied!';
      btn.classList.add('copied');
      setTimeout(function () {
        btn.textContent = 'Copy';
        btn.classList.remove('copied');
      }, 2000);
    } catch (e) {
      btn.textContent = 'Failed';
    }
    document.body.removeChild(ta);
  }

  function initKeyboardShortcuts() {
    document.addEventListener('keydown', function (e) {
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        var input = document.getElementById('searchInput');
        if (input) input.focus();
      }
      if (e.key === 'Escape') {
        var input = document.getElementById('searchInput');
        var results = document.getElementById('searchResults');
        if (input && document.activeElement === input) {
          input.blur();
          if (results) results.classList.remove('show');
        }
      }
      if (e.key === '/' && !e.ctrlKey && !e.metaKey && document.activeElement !== document.getElementById('searchInput')) {
        var tag = e.target.tagName;
        if (tag !== 'INPUT' && tag !== 'TEXTAREA' && tag !== 'SELECT') {
          e.preventDefault();
          var input = document.getElementById('searchInput');
          if (input) input.focus();
        }
      }
    });
  }

  function initFaqAccordion() {
    document.querySelectorAll('.faq-question').forEach(function (q) {
      q.addEventListener('click', function () {
        var item = q.parentElement;
        var wasOpen = item.classList.contains('open');
        item.classList.toggle('open');
        var answer = item.querySelector('.faq-answer');
        if (answer) {
          if (wasOpen) {
            answer.style.maxHeight = '0';
          } else {
            answer.style.maxHeight = answer.scrollHeight + 'px';
          }
        }
      });
    });
  }

  function initAnchorLinks() {
    document.querySelectorAll('.content h2[id], .content h3[id], .content h4[id]').forEach(function (h) {
      h.style.position = 'relative';
      var link = document.createElement('a');
      link.className = 'anchor-link';
      link.href = '#' + h.id;
      link.textContent = '#';
      link.style.cssText = 'position:absolute;left:-1.2em;opacity:0;text-decoration:none;color:var(--accent);font-weight:400;transition:opacity .15s';
      h.addEventListener('mouseenter', function () { link.style.opacity = '1'; });
      h.addEventListener('mouseleave', function () { link.style.opacity = '0'; });
      h.insertBefore(link, h.firstChild);
    });
  }

  // Re-init copy buttons and FAQ after dynamic content loads
  if (document.querySelector) {
    var observer = new MutationObserver(function () {
      initCopyButtons();
    });
    var content = document.querySelector('.content');
    if (content) {
      observer.observe(content, { childList: true, subtree: true });
    }
  }
})();
