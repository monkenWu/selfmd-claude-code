(function () {
    "use strict";

    var catalog = null;
    var pages = {};
    var currentLang = null;
    var docMeta = null;

    var uiI18n = {
        "zh-TW": {
            home: "首頁", overview: "概覽", pageNotFound: "頁面未找到",
            notFoundPrefix: "找不到", copyTitle: "複製本頁 .md 相對路徑",
            copyLabel: "複製本頁位置", dlTitle: "下載本頁 Markdown",
            dlLabel: "下載本頁", copied: "已複製", fullscreen: "全螢幕檢視",
            zoomHint: "滾輪縮放 / 拖曳平移", noResults: "找不到符合的結果",
            resultsPrefix: "找到 ", resultsSuffix: " 筆結果"
        },
        "en-US": {
            home: "Home", overview: "Overview", pageNotFound: "Page Not Found",
            notFoundPrefix: "Could not find", copyTitle: "Copy .md relative path",
            copyLabel: "Copy Path", dlTitle: "Download Markdown",
            dlLabel: "Download", copied: "Copied", fullscreen: "Fullscreen",
            zoomHint: "Scroll to zoom / Drag to pan", noResults: "No results found",
            resultsPrefix: "Found ", resultsSuffix: " results"
        }
    };

    function uiText(key) {
        var lang = currentLang || (docMeta && docMeta.default_language) || "zh-TW";
        var strings = uiI18n[lang] || uiI18n["en-US"];
        return strings[key] || (uiI18n["en-US"] && uiI18n["en-US"][key]) || key;
    }

    // ── Init ──

    function init() {
        if (!window.DOC_DATA) {
            document.getElementById("article").innerHTML = "<p>Error: _data.js not found.</p>";
            return;
        }

        catalog = window.DOC_DATA.catalog;
        pages = window.DOC_DATA.pages;
        docMeta = window.DOC_DATA.meta || null;

        configureMarked();
        mermaid.initialize({ startOnLoad: false, theme: "default" });

        if (docMeta && docMeta.available_languages && docMeta.available_languages.length > 1) {
            buildLangSwitcher();
            // Restore language from URL query string
            var urlLang = getQueryParam("lang");
            if (urlLang && urlLang !== docMeta.default_language) {
                switchLanguage(urlLang);
            }
        }

        buildSidebar();
        handleRoute();
        window.addEventListener("hashchange", handleRoute);
        setupMobileMenu();
        setupGlobalEsc();
        setupSearch();
    }

    // ── Language Switcher ──

    function buildLangSwitcher() {
        var container = document.getElementById("lang-switcher");
        if (!container || !docMeta) return;

        var label = document.createElement("label");
        label.className = "lang-label";
        label.textContent = "Language";
        label.setAttribute("for", "lang-select");
        container.appendChild(label);

        var select = document.createElement("select");
        select.id = "lang-select";
        select.className = "lang-select";

        for (var i = 0; i < docMeta.available_languages.length; i++) {
            var lang = docMeta.available_languages[i];
            var option = document.createElement("option");
            option.value = lang.code;
            option.textContent = lang.native_name;
            if (lang.is_default) option.selected = true;
            select.appendChild(option);
        }

        select.addEventListener("change", function () {
            switchLanguage(this.value);
        });

        container.appendChild(select);
    }

    function switchLanguage(langCode) {
        if (!docMeta) return;

        if (langCode === docMeta.default_language) {
            catalog = window.DOC_DATA.catalog;
            pages = window.DOC_DATA.pages;
            currentLang = null;
        } else {
            var langData = window.DOC_DATA.languages;
            if (langData && langData[langCode]) {
                if (langData[langCode].catalog) {
                    catalog = langData[langCode].catalog;
                }
                pages = langData[langCode].pages || {};
                currentLang = langCode;
            }
        }

        buildSidebar();
        handleRoute();

        var select = document.getElementById("lang-select");
        if (select) select.value = langCode;

        // Persist language in URL query string
        setQueryParam("lang", langCode === (docMeta && docMeta.default_language) ? null : langCode);
    }

    // ── Marked config ──

    function configureMarked() {
        marked.setOptions({
            highlight: function (code, lang) {
                // Skip highlight for mermaid — will be rendered by mermaid.js
                if (lang === "mermaid") {
                    return code.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
                }
                if (lang && hljs.getLanguage(lang)) {
                    return hljs.highlight(code, { language: lang }).value;
                }
                return hljs.highlightAuto(code).value;
            },
            breaks: false,
            gfm: true
        });
    }

    // ── Sidebar ──

    function buildSidebar() {
        var nav = document.getElementById("sidebar-nav");
        var html = '<ul>';
        html += '<li><a href="#index.md" data-path="index.md">' + uiText("home") + '</a></li>';

        for (var i = 0; i < catalog.items.length; i++) {
            html += renderSidebarSection(catalog.items[i], catalog.items[i].path);
        }

        html += '</ul>';
        nav.innerHTML = html;

        // Bind toggle events
        var toggles = nav.querySelectorAll(".section-toggle");
        for (var t = 0; t < toggles.length; t++) {
            toggles[t].addEventListener("click", onToggleSection);
        }
    }

    function renderSidebarSection(item, dirPath) {
        var pagePath = dirPath + "/index.md";
        var hasChildren = item.children && item.children.length > 0;

        var html = "<li>";

        if (hasChildren) {
            html += '<div class="section-toggle" data-path="' + pagePath + '">' + escapeHtml(item.title) + '</div>';
            html += '<div class="section-children">';
            html += '<ul>';
            html += '<li><a href="#' + pagePath + '" data-path="' + pagePath + '">' + uiText("overview") + '</a></li>';
            for (var i = 0; i < item.children.length; i++) {
                var child = item.children[i];
                var childDir = dirPath + "/" + child.path;
                if (child.children && child.children.length > 0) {
                    html += renderSidebarSection(child, childDir);
                } else {
                    var childPage = childDir + "/index.md";
                    html += '<li><a href="#' + childPage + '" data-path="' + childPage + '">' + escapeHtml(child.title) + '</a></li>';
                }
            }
            html += '</ul>';
            html += '</div>';
        } else {
            html += '<a href="#' + pagePath + '" data-path="' + pagePath + '">' + escapeHtml(item.title) + '</a>';
        }

        html += "</li>";
        return html;
    }

    function onToggleSection(e) {
        var toggle = e.currentTarget;
        var children = toggle.nextElementSibling;

        if (toggle.classList.contains("expanded")) {
            toggle.classList.remove("expanded");
            children.classList.remove("show");
        } else {
            toggle.classList.add("expanded");
            children.classList.add("show");
        }
    }

    // ── Routing ──

    function handleRoute() {
        var path = location.hash.slice(1) || "index.md";
        loadPage(path);
    }

    function loadPage(path) {
        var md = pages[path];
        if (!md) {
            document.getElementById("article").innerHTML =
                "<h1>" + uiText("pageNotFound") + "</h1><p>" + uiText("notFoundPrefix") + " <code>" + escapeHtml(path) + "</code></p>";
            return;
        }

        renderContent(md, path);
        updateActiveLink(path);
        updateTitle(path);

        // Scroll to match or top
        if (pendingSearchHighlight) {
            scrollToMatch(pendingSearchHighlight);
            pendingSearchHighlight = null;
        } else {
            document.getElementById("content").scrollTop = 0;
        }

        // Close mobile menu
        document.getElementById("sidebar").classList.remove("open");
        var overlay = document.querySelector(".sidebar-overlay");
        if (overlay) overlay.remove();
    }

    // ── Render ──

    function renderContent(md, currentPath) {
        var article = document.getElementById("article");

        var html = marked.parse(md);
        article.innerHTML = buildPageToolbar(currentPath) + html;
        fixLinks(article, currentPath);
        renderMermaid(article);

        // Bind toolbar buttons
        var copyBtn = document.getElementById("btn-copy-path");
        if (copyBtn) {
            copyBtn.addEventListener("click", function () {
                copyToClipboard(currentPath, copyBtn);
            });
        }
        var dlBtn = document.getElementById("btn-download");
        if (dlBtn) {
            dlBtn.addEventListener("click", function () {
                downloadPage(currentPath, md);
            });
        }
    }

    // ── Page Toolbar ──

    function buildPageToolbar(path) {
        return '<div class="page-toolbar">' +
            '<button id="btn-copy-path" class="toolbar-btn" title="' + uiText("copyTitle") + '">' +
            '<span class="toolbar-icon">&#128203;</span> ' + uiText("copyLabel") + '</button>' +
            '<button id="btn-download" class="toolbar-btn" title="' + uiText("dlTitle") + '">' +
            '<span class="toolbar-icon">&#11015;</span> ' + uiText("dlLabel") + '</button>' +
            '</div>';
    }

    function copyToClipboard(text, btn) {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(text).then(function () {
                showCopyFeedback(btn);
            }, function () {
                fallbackCopy(text, btn);
            });
        } else {
            fallbackCopy(text, btn);
        }
    }

    function fallbackCopy(text, btn) {
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand("copy"); showCopyFeedback(btn); } catch (e) { /* ignore */ }
        document.body.removeChild(ta);
    }

    function showCopyFeedback(btn) {
        var orig = btn.innerHTML;
        btn.innerHTML = '<span class="toolbar-icon">&#10003;</span> ' + uiText("copied");
        btn.classList.add("toolbar-btn-success");
        setTimeout(function () {
            btn.innerHTML = orig;
            btn.classList.remove("toolbar-btn-success");
        }, 1500);
    }

    function downloadPage(path, md) {
        var filename = path.replace(/\//g, "_");
        var blob = new Blob([md], { type: "text/markdown;charset=utf-8" });
        var url = URL.createObjectURL(blob);
        var a = document.createElement("a");
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }

    function fixLinks(container, currentPath) {
        // Get base directory of current page
        var baseDir = currentPath.replace(/[^/]*$/, "");
        var links = container.querySelectorAll("a[href]");

        for (var i = 0; i < links.length; i++) {
            var href = links[i].getAttribute("href");
            // Skip external links, anchor links, and already-hashed links
            if (href.indexOf("://") !== -1 || href.charAt(0) === "#" || href.indexOf("mailto:") === 0) continue;
            // Resolve relative path and convert to hash link
            var resolved = resolvePath(baseDir, href);
            links[i].setAttribute("href", "#" + resolved);
        }
    }

    function resolvePath(base, relative) {
        var combined = base + relative;
        var parts = combined.split("/");
        var resolved = [];

        for (var i = 0; i < parts.length; i++) {
            if (parts[i] === "..") {
                resolved.pop();
            } else if (parts[i] !== "." && parts[i] !== "") {
                resolved.push(parts[i]);
            }
        }

        return resolved.join("/");
    }

    function renderMermaid(container) {
        var codeBlocks = container.querySelectorAll("pre code.language-mermaid");
        if (codeBlocks.length === 0) return;

        for (var i = 0; i < codeBlocks.length; i++) {
            var block = codeBlocks[i];
            var pre = block.parentElement;

            var wrapper = document.createElement("div");
            wrapper.className = "mindmap-wrapper";

            var div = document.createElement("div");
            div.className = "mermaid";
            // Decode HTML entities (e.g. &lt; → <) and normalize \n → <br/> for all diagram types
            var mermaidText = decodeHtmlEntities(block.textContent).replace(/\\n/g, "<br/>");
            div.textContent = mermaidText;

            var btn = document.createElement("button");
            btn.className = "mindmap-fullscreen-btn";
            btn.title = uiText("fullscreen");
            btn.innerHTML = "&#x26F6;";

            wrapper.appendChild(div);
            wrapper.appendChild(btn);
            pre.parentNode.replaceChild(wrapper, pre);
        }

        try {
            mermaid.run();
        } catch (e) {
            console.warn("Mermaid rendering error:", e);
        }

        // Bind fullscreen to each wrapper
        var btns = container.querySelectorAll(".mindmap-fullscreen-btn");
        for (var b = 0; b < btns.length; b++) {
            (function (btn) {
                btn.addEventListener("click", function () {
                    var wrapper = btn.closest(".mindmap-wrapper");
                    if (!wrapper) return;
                    if (wrapper.classList.contains("mindmap-fullscreen")) {
                        exitMindmapFullscreen(wrapper, btn);
                    } else {
                        enterMindmapFullscreen(wrapper, btn);
                    }
                });
            })(btns[b]);
        }
    }

    // ── Active state ──

    function updateActiveLink(path) {
        var nav = document.getElementById("sidebar-nav");

        // Clear all active states
        var actives = nav.querySelectorAll("a.active, .section-toggle.active");
        for (var i = 0; i < actives.length; i++) {
            actives[i].classList.remove("active");
        }

        // Activate matching link
        var link = nav.querySelector('a[data-path="' + path + '"]');
        if (link) {
            link.classList.add("active");
            // Expand parent sections
            expandParents(link);
        }

        // Also check section toggles
        var toggle = nav.querySelector('.section-toggle[data-path="' + path + '"]');
        if (toggle) {
            toggle.classList.add("active");
            expandParents(toggle);
        }
    }

    function expandParents(el) {
        var parent = el.parentElement;
        while (parent && parent.id !== "sidebar-nav") {
            if (parent.classList.contains("section-children")) {
                parent.classList.add("show");
                var toggle = parent.previousElementSibling;
                if (toggle && toggle.classList.contains("section-toggle")) {
                    toggle.classList.add("expanded");
                }
            }
            parent = parent.parentElement;
        }
    }

    function updateTitle(path) {
        var name = document.getElementById("project-name").textContent;
        if (path === "index.md") {
            document.title = name + " — Documentation";
        } else {
            // Extract title from first # heading
            var md = pages[path];
            if (md) {
                var match = md.match(/^#\s+(.+)/m);
                if (match) {
                    document.title = match[1] + " — " + name;
                }
            }
        }
    }

    // ── Mobile ──

    function setupMobileMenu() {
        var btn = document.getElementById("menu-toggle");
        btn.addEventListener("click", function () {
            var sidebar = document.getElementById("sidebar");
            var isOpen = sidebar.classList.contains("open");

            if (isOpen) {
                sidebar.classList.remove("open");
                var overlay = document.querySelector(".sidebar-overlay");
                if (overlay) overlay.remove();
            } else {
                sidebar.classList.add("open");
                var overlay = document.createElement("div");
                overlay.className = "sidebar-overlay";
                overlay.addEventListener("click", function () {
                    sidebar.classList.remove("open");
                    overlay.remove();
                });
                document.body.appendChild(overlay);
            }
        });
    }

    // ── Mindmap fullscreen + zoom/pan ──

    var mmState = { scale: 1, tx: 0, ty: 0, dragging: false, lastX: 0, lastY: 0 };

    function resetMmState() {
        mmState.scale = 1; mmState.tx = 0; mmState.ty = 0;
    }

    function applyMmTransform() {
        var inner = document.querySelector(".mindmap-wrapper.mindmap-fullscreen .mindmap-inner");
        if (!inner) return;
        inner.style.transform = "translate(" + mmState.tx + "px," + mmState.ty + "px) scale(" + mmState.scale + ")";
    }

    function setupGlobalEsc() {
        document.addEventListener("keydown", function (e) {
            if (e.key === "Escape") {
                var wrapper = document.querySelector(".mindmap-wrapper.mindmap-fullscreen");
                if (wrapper) {
                    var b = wrapper.querySelector(".mindmap-fullscreen-btn");
                    exitMindmapFullscreen(wrapper, b);
                }
            }
        });
    }

    function enterMindmapFullscreen(wrapper, btn) {
        // Wrap mermaid content in an inner div for transform
        var mermaidDiv = wrapper.querySelector(".mermaid");
        if (mermaidDiv && !wrapper.querySelector(".mindmap-inner")) {
            var inner = document.createElement("div");
            inner.className = "mindmap-inner";
            mermaidDiv.parentNode.insertBefore(inner, mermaidDiv);
            inner.appendChild(mermaidDiv);
        }

        resetMmState();
        wrapper.classList.add("mindmap-fullscreen");
        if (btn) btn.innerHTML = "&#x2715;";
        document.body.style.overflow = "hidden";

        // Zoom hint
        var hint = wrapper.querySelector(".mindmap-hint");
        if (!hint) {
            hint = document.createElement("div");
            hint.className = "mindmap-hint";
            hint.textContent = uiText("zoomHint");
            wrapper.appendChild(hint);
            setTimeout(function () { hint.classList.add("fade-out"); }, 2000);
            setTimeout(function () { if (hint.parentNode) hint.parentNode.removeChild(hint); }, 2800);
        }

        // Bind zoom & pan
        wrapper.addEventListener("wheel", onMmWheel, { passive: false });
        wrapper.addEventListener("mousedown", onMmMouseDown);
        wrapper.addEventListener("mousemove", onMmMouseMove);
        wrapper.addEventListener("mouseup", onMmMouseUp);
        wrapper.addEventListener("mouseleave", onMmMouseUp);
    }

    function exitMindmapFullscreen(wrapper, btn) {
        wrapper.classList.remove("mindmap-fullscreen");
        if (btn) btn.innerHTML = "&#x26F6;";
        document.body.style.overflow = "";
        resetMmState();

        // Unwrap inner div
        var inner = wrapper.querySelector(".mindmap-inner");
        if (inner) {
            var mermaidDiv = inner.querySelector(".mermaid");
            if (mermaidDiv) {
                inner.parentNode.insertBefore(mermaidDiv, inner);
                inner.parentNode.removeChild(inner);
            }
        }

        wrapper.removeEventListener("wheel", onMmWheel);
        wrapper.removeEventListener("mousedown", onMmMouseDown);
        wrapper.removeEventListener("mousemove", onMmMouseMove);
        wrapper.removeEventListener("mouseup", onMmMouseUp);
        wrapper.removeEventListener("mouseleave", onMmMouseUp);
    }

    function onMmWheel(e) {
        e.preventDefault();
        var delta = e.deltaY > 0 ? 0.95 : 1.05;
        var newScale = mmState.scale * delta;
        if (newScale < 0.8) newScale = 0.8;
        if (newScale > 2.0) newScale = 2.0;
        mmState.scale = newScale;
        applyMmTransform();
    }

    function onMmMouseDown(e) {
        if (e.target.closest(".mindmap-fullscreen-btn")) return;
        mmState.dragging = true;
        mmState.lastX = e.clientX;
        mmState.lastY = e.clientY;
        e.currentTarget.style.cursor = "grabbing";
    }

    function onMmMouseMove(e) {
        if (!mmState.dragging) return;
        mmState.tx += e.clientX - mmState.lastX;
        mmState.ty += e.clientY - mmState.lastY;
        mmState.lastX = e.clientX;
        mmState.lastY = e.clientY;
        applyMmTransform();
    }

    function onMmMouseUp(e) {
        mmState.dragging = false;
        var wrapper = document.querySelector(".mindmap-wrapper.mindmap-fullscreen");
        if (wrapper) wrapper.style.cursor = "grab";
    }

    // ── Search ──

    var searchTimer = null;
    var pendingSearchHighlight = null;

    function setupSearch() {
        var input = document.getElementById("search-input");
        if (!input) return;

        input.addEventListener("input", function () {
            clearTimeout(searchTimer);
            var query = input.value.trim();
            if (!query) {
                hideSearchResults();
                return;
            }
            searchTimer = setTimeout(function () {
                doSearch(query);
            }, 200);
        });

        input.addEventListener("keydown", function (e) {
            if (e.key === "Escape") {
                input.value = "";
                hideSearchResults();
            }
        });
    }

    function doSearch(query) {
        var results = [];
        var lowerQuery = query.toLowerCase();
        var pageKeys = Object.keys(pages);

        for (var i = 0; i < pageKeys.length; i++) {
            var path = pageKeys[i];
            var content = pages[path];
            var lowerContent = content.toLowerCase();
            var idx = lowerContent.indexOf(lowerQuery);
            if (idx === -1) continue;

            // Extract title from first heading
            var titleMatch = content.match(/^#\s+(.+)/m);
            var title = titleMatch ? titleMatch[1] : path;

            // Extract snippet around match
            var start = Math.max(0, idx - 30);
            var end = Math.min(content.length, idx + query.length + 60);
            var snippet = (start > 0 ? "…" : "") +
                content.substring(start, end).replace(/\n/g, " ") +
                (end < content.length ? "…" : "");

            results.push({ path: path, title: title, snippet: snippet, matchIdx: idx - start + (start > 0 ? 1 : 0) });
        }

        showSearchResults(results, query);
    }

    function showSearchResults(results, query) {
        var container = document.getElementById("search-results");
        var nav = document.getElementById("sidebar-nav");
        if (!container) return;

        if (results.length === 0) {
            container.innerHTML = '<div class="search-empty">' + uiText("noResults") + '</div>';
            container.style.display = "block";
            nav.style.display = "none";
            return;
        }

        var html = '<div class="search-count">' + uiText("resultsPrefix") + results.length + uiText("resultsSuffix") + '</div>';
        for (var i = 0; i < results.length; i++) {
            var r = results[i];
            html += '<a class="search-result-item" href="#' + escapeHtml(r.path) + '">' +
                '<div class="search-result-title">' + escapeHtml(r.title) + '</div>' +
                '<div class="search-result-snippet">' + highlightText(escapeHtml(r.snippet), query) + '</div>' +
                '</a>';
        }

        container.innerHTML = html;
        container.style.display = "block";
        nav.style.display = "none";

        // Clicking a result clears search and highlights match in content
        var items = container.querySelectorAll(".search-result-item");
        for (var j = 0; j < items.length; j++) {
            (function (item) {
                item.addEventListener("click", function () {
                    pendingSearchHighlight = query;
                    document.getElementById("search-input").value = "";
                    hideSearchResults();
                });
            })(items[j]);
        }
    }

    function hideSearchResults() {
        var container = document.getElementById("search-results");
        var nav = document.getElementById("sidebar-nav");
        if (container) container.style.display = "none";
        if (nav) nav.style.display = "";
    }

    function highlightText(text, query) {
        var lowerText = text.toLowerCase();
        var lowerQuery = query.toLowerCase();
        var idx = lowerText.indexOf(lowerQuery);
        if (idx === -1) return text;
        return text.substring(0, idx) +
            '<mark>' + text.substring(idx, idx + query.length) + '</mark>' +
            text.substring(idx + query.length);
    }

    function scrollToMatch(query) {
        var article = document.getElementById("article");
        var contentEl = document.getElementById("content");
        if (!article || !contentEl) return;

        // Use TreeWalker to find the text node containing the query
        var walker = document.createTreeWalker(article, NodeFilter.SHOW_TEXT, null, false);
        var lowerQuery = query.toLowerCase();
        var node;
        while ((node = walker.nextNode())) {
            if (node.nodeValue.toLowerCase().indexOf(lowerQuery) !== -1) {
                // Wrap the match in a <mark> for visual highlight
                var idx = node.nodeValue.toLowerCase().indexOf(lowerQuery);
                var range = document.createRange();
                range.setStart(node, idx);
                range.setEnd(node, idx + query.length);
                var mark = document.createElement("mark");
                mark.className = "search-highlight";
                range.surroundContents(mark);

                // Scroll into view
                var markTop = mark.getBoundingClientRect().top;
                var contentRect = contentEl.getBoundingClientRect();
                contentEl.scrollTop = contentEl.scrollTop + (markTop - contentRect.top) - 80;

                // Remove highlight after a moment
                setTimeout(function () {
                    if (mark.parentNode) {
                        mark.outerHTML = mark.innerHTML;
                    }
                }, 2000);
                return;
            }
        }
    }

    // ── Utils ──

    function getQueryParam(key) {
        var params = new URLSearchParams(window.location.search);
        return params.get(key);
    }

    function setQueryParam(key, value) {
        var params = new URLSearchParams(window.location.search);
        if (value) {
            params.set(key, value);
        } else {
            params.delete(key);
        }
        var qs = params.toString();
        var newUrl = window.location.pathname + (qs ? "?" + qs : "") + window.location.hash;
        window.history.replaceState(null, "", newUrl);
    }

    function escapeHtml(text) {
        var div = document.createElement("div");
        div.appendChild(document.createTextNode(text));
        return div.innerHTML;
    }

    function decodeHtmlEntities(text) {
        var ta = document.createElement("textarea");
        ta.innerHTML = text;
        return ta.value;
    }

    // ── Start ──

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    } else {
        init();
    }
})();
