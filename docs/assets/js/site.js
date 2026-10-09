/* Data/AI on ACK — site interactions. Progressive enhancement only. */
(function () {
  "use strict";

  var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var isCoarse = window.matchMedia("(pointer: coarse)").matches;
  var $ = function (s, c) { return (c || document).querySelector(s); };
  var $$ = function (s, c) { return Array.prototype.slice.call((c || document).querySelectorAll(s)); };

  /* ---------- Header scroll state + progress ---------- */
  var header = $("#siteHeader");
  var progress = $("#scrollProgress");
  function onScroll() {
    if (header) header.classList.toggle("is-scrolled", window.scrollY > 12);
    if (progress) {
      var h = document.documentElement;
      var max = h.scrollHeight - h.clientHeight;
      progress.style.transform = "scaleX(" + (max > 0 ? Math.min(window.scrollY / max, 1) : 0) + ")";
    }
  }
  window.addEventListener("scroll", onScroll, { passive: true });
  onScroll();

  /* ---------- Mobile sidebar ---------- */
  var menuToggle = $("#menuToggle");
  var sidebar = $("#docsSidebar");
  var backdrop = $("#docsBackdrop");
  function closeMenu() {
    if (!sidebar) return;
    sidebar.classList.remove("open");
    if (backdrop) { backdrop.classList.remove("show"); backdrop.hidden = true; }
    if (menuToggle) menuToggle.setAttribute("aria-expanded", "false");
    document.body.style.overflow = "";
  }
  var mobileNav = $("#mobileNav");
  if (menuToggle && sidebar) {
    menuToggle.addEventListener("click", function () {
      var open = sidebar.classList.toggle("open");
      menuToggle.setAttribute("aria-expanded", String(open));
      if (backdrop) { backdrop.hidden = !open; if (open) requestAnimationFrame(function () { backdrop.classList.add("show"); }); }
      document.body.style.overflow = open ? "hidden" : "";
    });
    if (backdrop) backdrop.addEventListener("click", closeMenu);
    document.addEventListener("keydown", function (e) { if (e.key === "Escape") closeMenu(); });
    sidebar.addEventListener("click", function (e) { if (e.target.closest("a")) closeMenu(); });
  } else if (menuToggle && mobileNav) {
    /* Pages without a docs sidebar (e.g. the landing page) get a dropdown nav. */
    menuToggle.addEventListener("click", function () {
      var open = document.body.classList.toggle("mobile-nav-open");
      menuToggle.setAttribute("aria-expanded", String(open));
    });
    mobileNav.addEventListener("click", function (e) {
      if (e.target.closest("a")) {
        document.body.classList.remove("mobile-nav-open");
        menuToggle.setAttribute("aria-expanded", "false");
      }
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") {
        document.body.classList.remove("mobile-nav-open");
        menuToggle.setAttribute("aria-expanded", "false");
      }
    });
  }

  /* ---------- Reveal on scroll ---------- */
  var revealEls = $$(".reveal");
  if (revealEls.length && "IntersectionObserver" in window && !reduceMotion) {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (en) {
        if (en.isIntersecting) { en.target.classList.add("in"); io.unobserve(en.target); }
      });
    }, { threshold: 0.12, rootMargin: "0px 0px -6% 0px" });
    revealEls.forEach(function (el) { io.observe(el); });
  } else {
    revealEls.forEach(function (el) { el.classList.add("in"); });
  }

  /* ---------- Aurora canvas (hero) ---------- */
  var canvas = $("#auroraCanvas");
  if (canvas && !reduceMotion) {
    var ctx = canvas.getContext("2d");
    var W, H, dpr, blobs, raf = null, t0 = performance.now();
    var COLORS = [[79, 140, 255], [124, 92, 255], [180, 92, 255], [76, 201, 240]];
    function resize() {
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      W = canvas.clientWidth; H = canvas.clientHeight;
      canvas.width = W * dpr; canvas.height = H * dpr;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    }
    function makeBlobs() {
      blobs = COLORS.map(function (c, i) {
        return {
          c: c,
          r: 0.32 + 0.1 * (i % 2),
          ax: 0.5 + 0.22 * Math.cos(i * 1.7),
          ay: 0.34 + 0.2 * Math.sin(i * 2.3),
          sx: 0.16 + 0.05 * i, sy: 0.12 + 0.04 * i,
          p: i * 1.9
        };
      });
    }
    function frame(now) {
      var t = (now - t0) / 1000;
      ctx.clearRect(0, 0, W, H);
      ctx.globalCompositeOperation = "lighter";
      blobs.forEach(function (b) {
        var x = W * (b.ax + b.sx * Math.cos(t * 0.14 + b.p));
        var y = H * (b.ay + b.sy * Math.sin(t * 0.11 + b.p * 1.3));
        var r = Math.max(W, H) * b.r;
        var g = ctx.createRadialGradient(x, y, 0, x, y, r);
        g.addColorStop(0, "rgba(" + b.c[0] + "," + b.c[1] + "," + b.c[2] + ",0.30)");
        g.addColorStop(1, "rgba(" + b.c[0] + "," + b.c[1] + "," + b.c[2] + ",0)");
        ctx.fillStyle = g;
        ctx.fillRect(x - r, y - r, r * 2, r * 2);
      });
      raf = requestAnimationFrame(frame);
    }
    function start() { if (raf === null) raf = requestAnimationFrame(frame); }
    function stop() { if (raf !== null) { cancelAnimationFrame(raf); raf = null; } }
    resize(); makeBlobs(); start();
    window.addEventListener("resize", function () { resize(); }, { passive: true });
    document.addEventListener("visibilitychange", function () { document.hidden ? stop() : start(); });
  }

  /* ---------- Count-up stats ---------- */
  $$("[data-count]").forEach(function (el) {
    var target = parseInt(el.getAttribute("data-count"), 10);
    if (isNaN(target) || reduceMotion) { if (!isNaN(target)) el.textContent = String(target); return; }
    var started = false;
    var io2 = new IntersectionObserver(function (entries) {
      if (!entries[0].isIntersecting || started) return;
      started = true; io2.disconnect();
      var t0 = performance.now(), dur = 1400;
      (function tick(now) {
        var p = Math.min((now - t0) / dur, 1);
        el.textContent = String(Math.round(target * (1 - Math.pow(1 - p, 3))));
        if (p < 1) requestAnimationFrame(tick);
      })(t0);
    }, { threshold: 0.4 });
    io2.observe(el);
  });

  /* ---------- Tilt cards ---------- */
  if (!reduceMotion && !isCoarse) {
    $$("[data-tilt]").forEach(function (card) {
      var rafId = null;
      card.addEventListener("pointermove", function (e) {
        if (rafId) return;
        rafId = requestAnimationFrame(function () {
          var r = card.getBoundingClientRect();
          var px = (e.clientX - r.left) / r.width - 0.5;
          var py = (e.clientY - r.top) / r.height - 0.5;
          card.style.transform = "perspective(900px) rotateX(" + (-py * 4).toFixed(2) + "deg) rotateY(" + (px * 5).toFixed(2) + "deg) translateY(-2px)";
          rafId = null;
        });
      });
      card.addEventListener("pointerleave", function () {
        if (rafId) { cancelAnimationFrame(rafId); rafId = null; }
        card.style.transform = "";
      });
    });
    /* Pointer glow for capability cards */
    $$(".cap-card").forEach(function (card) {
      card.addEventListener("pointermove", function (e) {
        var r = card.getBoundingClientRect();
        card.style.setProperty("--mx", (e.clientX - r.left) + "px");
        card.style.setProperty("--my", (e.clientY - r.top) + "px");
      });
    });
  }

  /* ---------- Copy buttons ---------- */
  function wireCopy(btn, getText) {
    btn.addEventListener("click", function () {
      var done = function () {
        btn.classList.add("copied");
        setTimeout(function () { btn.classList.remove("copied"); }, 1600);
      };
      var text = getText();
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, done);
      } else {
        var ta = document.createElement("textarea");
        ta.value = text; ta.style.position = "fixed"; ta.style.opacity = "0";
        document.body.appendChild(ta); ta.select();
        try { document.execCommand("copy"); } catch (e) {}
        document.body.removeChild(ta); done();
      }
    });
  }
  $$("[data-copy-target]").forEach(function (btn) {
    var target = $(btn.getAttribute("data-copy-target"));
    if (!target) return;
    wireCopy(btn, function () {
      return target.innerText.replace(/^\$ /gm, "").replace(/\n{3,}/g, "\n\n").trim();
    });
  });
  $$(".prose pre").forEach(function (pre) {
    if (pre.querySelector(".code-copy-btn")) return;
    var btn = document.createElement("button");
    btn.type = "button"; btn.className = "code-copy-btn"; btn.setAttribute("aria-label", "复制代码");
    btn.innerHTML = '<svg class="icon-copy"><use href="#i-copy"/></svg><svg class="icon-ok"><use href="#i-check"/></svg>';
    pre.appendChild(btn);
    wireCopy(btn, function () {
      var code = pre.querySelector("code");
      return (code ? code.innerText : pre.innerText).trim();
    });
  });

  /* ---------- Table wrapping ---------- */
  $$(".prose table").forEach(function (table) {
    if (table.parentNode.classList && table.parentNode.classList.contains("table-wrap")) return;
    var wrap = document.createElement("div");
    wrap.className = "table-wrap";
    table.parentNode.insertBefore(wrap, table);
    wrap.appendChild(table);
  });

  /* ---------- TOC + scrollspy ---------- */
  var prose = $("#prose");
  var tocNav = $("#tocNav");
  if (prose && tocNav) {
    var headings = $$("h2, h3", prose).filter(function (h) { return h.id; });
    if (headings.length >= 2) {
      headings.forEach(function (h) {
        var a = document.createElement("a");
        a.href = "#" + h.id;
        a.textContent = h.textContent;
        a.className = h.tagName === "H3" ? "toc-h3" : "";
        tocNav.appendChild(a);
      });
      var links = $$("a", tocNav);
      var spy = new IntersectionObserver(function (entries) {
        entries.forEach(function (en) {
          if (en.isIntersecting) {
            links.forEach(function (l) { l.classList.toggle("is-active", l.hash === "#" + en.target.id); });
          }
        });
      }, { rootMargin: "-80px 0px -66% 0px", threshold: 0 });
      headings.forEach(function (h) { spy.observe(h); });
    } else {
      var toc = $("#docsToc");
      if (toc) toc.style.display = "none";
    }
  }

  /* ---------- Sidebar: scroll active into view ---------- */
  var activeLink = $(".side-nav .nav-link.is-active");
  if (activeLink && sidebar && window.innerWidth > 960) {
    requestAnimationFrame(function () {
      activeLink.scrollIntoView({ block: "center" });
    });
  }

  /* ---------- Prev / Next pager ---------- */
  var pager = $("#docPager");
  if (pager) {
    var navLinks = $$(".side-nav .nav-link").filter(function (a) {
      return a.getAttribute("href") && !a.classList.contains("nav-top");
    });
    var current = location.pathname.replace(/index\.html$/, "").replace(/\/$/, "");
    var idx = -1;
    navLinks.forEach(function (a, i) {
      var path = new URL(a.href, location.origin).pathname.replace(/index\.html$/, "").replace(/\/$/, "");
      if (path === current) idx = i;
    });
    if (idx > -1) {
      var prev = navLinks[idx - 1], next = navLinks[idx + 1];
      var html = "";
      if (prev) html += '<a class="pager-link pager-prev" href="' + prev.getAttribute("href") + '"><small>← 上一篇</small><span>' + prev.textContent.trim() + "</span></a>";
      else html += "<span></span>";
      if (next) html += '<a class="pager-link pager-next" href="' + next.getAttribute("href") + '"><small>下一篇 →</small><span>' + next.textContent.trim() + "</span></a>";
      pager.innerHTML = html;
    }
  }

  /* ---------- GitHub stars (best effort) ---------- */
  function fmtStars(n) { return n >= 1000 ? (n / 1000).toFixed(1).replace(/\.0$/, "") + "k" : String(n); }
  try {
    var cached = sessionStorage.getItem("gh-stars-data-on-ack");
    if (cached) paintStars(parseInt(cached, 10));
    fetch("https://api.github.com/repos/AliyunContainerService/data-on-ack")
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (d) {
        if (d && typeof d.stargazers_count === "number") {
          sessionStorage.setItem("gh-stars-data-on-ack", String(d.stargazers_count));
          paintStars(d.stargazers_count);
        }
      })
      .catch(function () {});
  } catch (e) {}
  function paintStars(n) {
    if (isNaN(n)) return;
    var a = $("#githubStars"); if (a) { a.textContent = fmtStars(n); a.hidden = false; }
    var hb = $("#heroStars"), hc = $("#heroStarsCount");
    if (hb && hc) { hc.textContent = fmtStars(n) + " stars"; hb.hidden = false; }
  }

  /* ---------- Search palette ---------- */
  var overlay = null, input = null, resultsBox = null, indexData = null, indexPromise = null, selected = 0;
  var INDEX_URL = document.querySelector('link[rel="search-index"]');
  INDEX_URL = INDEX_URL ? INDEX_URL.href : null;

  function buildOverlay() {
    overlay = document.createElement("div");
    overlay.className = "search-overlay";
    overlay.innerHTML =
      '<div class="search-panel" role="dialog" aria-modal="true" aria-label="搜索文档">' +
        '<div class="search-input-row">' +
          '<svg><use href="#i-search"/></svg>' +
          '<input type="text" placeholder="搜索文档、指南、实践…" autocomplete="off" spellcheck="false">' +
          "<kbd>ESC</kbd>" +
        "</div>" +
        '<div class="search-results"></div>' +
        '<div class="search-foot"><span><kbd>↑</kbd><kbd>↓</kbd> 移动</span><span><kbd>↵</kbd> 打开</span><span><kbd>esc</kbd> 关闭</span></div>' +
      "</div>";
    document.body.appendChild(overlay);
    input = overlay.querySelector("input");
    resultsBox = overlay.querySelector(".search-results");
    overlay.addEventListener("click", function (e) { if (e.target === overlay) closeSearch(); });
    input.addEventListener("input", function () { renderResults(input.value.trim()); });
    input.addEventListener("keydown", function (e) {
      var items = $$(".search-result", resultsBox);
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        if (!items.length) return;
        selected = (selected + (e.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
        items.forEach(function (el, i) { el.classList.toggle("is-selected", i === selected); });
        items[selected].scrollIntoView({ block: "nearest" });
      } else if (e.key === "Enter" && items[selected]) {
        location.href = items[selected].href;
      }
    });
  }
  function openSearch() {
    if (!overlay) buildOverlay();
    overlay.classList.add("open");
    document.body.style.overflow = "hidden";
    input.value = ""; renderResults("");
    setTimeout(function () { input.focus(); }, 60);
    if (!indexPromise && INDEX_URL) {
      indexPromise = fetch(INDEX_URL).then(function (r) { return r.ok ? r.json() : []; })
        .then(function (d) {
          d.forEach(function (p) { p.excerpt = (p.excerpt || "").replace(/\{%[^%]*%\}/g, "").replace(/\s{2,}/g, " "); });
          indexData = d; renderResults(input.value.trim());
        })
        .catch(function () { indexData = []; });
    }
  }
  function closeSearch() {
    if (!overlay) return;
    overlay.classList.remove("open");
    document.body.style.overflow = "";
  }
  function esc(s) { return s.replace(/[&<>"]/g, function (c) { return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]; }); }
  function renderResults(q) {
    if (!resultsBox) return;
    if (!indexData) {
      resultsBox.innerHTML = '<div class="search-empty">正在加载索引…</div>';
      return;
    }
    var ql = q.toLowerCase();
    var hits = !ql ? indexData.slice(0, 8) : indexData.filter(function (p) {
      return (p.title + " " + (p.parent || "") + " " + p.excerpt).toLowerCase().indexOf(ql) > -1;
    }).slice(0, 12);
    if (!hits.length) {
      resultsBox.innerHTML = '<div class="search-empty">没有找到与「' + esc(q) + '」相关的内容</div>';
      return;
    }
    selected = 0;
    resultsBox.innerHTML = hits.map(function (p, i) {
      var title = esc(p.title), excerpt = esc(p.excerpt || "");
      if (ql) {
        var hi = function (s) {
          var pos = s.toLowerCase().indexOf(ql);
          return pos > -1 ? s.slice(0, pos) + "<mark>" + s.slice(pos, pos + ql.length) + "</mark>" + s.slice(pos + ql.length) : s;
        };
        title = hi(title); excerpt = hi(excerpt);
      }
      return '<a class="search-result' + (i === 0 ? " is-selected" : "") + '" href="' + p.url + '">' +
        (p.parent ? '<span class="sr-path">' + esc(p.parent) + "</span>" : "") +
        '<span class="sr-title">' + title + "</span>" +
        '<span class="sr-excerpt">' + excerpt + "</span></a>";
    }).join("");
  }
  var trigger = $("#searchTrigger");
  if (trigger) trigger.addEventListener("click", openSearch);
  document.addEventListener("keydown", function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); openSearch(); }
    if (e.key === "Escape") closeSearch();
  });
  if (!/Mac|iPhone|iPad/.test(navigator.platform || "")) {
    var kbd = $("#searchKbd"); if (kbd) kbd.textContent = "Ctrl K";
  }
})();
