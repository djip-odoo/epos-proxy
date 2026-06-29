(function () {
    'use strict';

    const app = document.getElementById('app');

    function htmlToFragment(html) {
        const t = document.createElement('template');
        t.innerHTML = html.trim();
        return t.content;
    }

    function esc(str) {
        const d = document.createElement('div');
        d.textContent = str;
        return d.innerHTML;
    }

    function renderItem(item) {
        if (typeof item === 'string') {
            return htmlToFragment(processInlineCode(item));
        }
        if (item.html) {
            return htmlToFragment(item.html);
        }
        if (item.list) {
            return renderListBlock(item.list);
        }
        return document.createDocumentFragment();
    }

    function processInlineCode(text) {
        return text.replace(/`([^`]+)`/g, '<code>$1</code>');
    }

    function renderListBlock(block) {
        const tag = block.ordered ? 'ol' : 'ul';
        const el = document.createElement(tag);
        block.items.forEach(function (it) {
            const li = document.createElement('li');
            const frag = renderItem(it);
            li.appendChild(frag);
            el.appendChild(li);
        });
        return el;
    }

    function renderTableBlock(block) {
        const table = document.createElement('table');
        const thead = document.createElement('thead');
        const htr = document.createElement('tr');
        block.headers.forEach(function (h) {
            const th = document.createElement('th');
            th.textContent = h;
            htr.appendChild(th);
        });
        thead.appendChild(htr);
        table.appendChild(thead);

        const tbody = document.createElement('tbody');
        block.rows.forEach(function (row) {
            const tr = document.createElement('tr');
            row.forEach(function (cell) {
                const td = document.createElement('td');
                td.textContent = cell;
                tr.appendChild(td);
            });
            tbody.appendChild(tr);
        });
        table.appendChild(tbody);
        return table;
    }

    function renderImageBlock(block) {
        const container = document.createElement('div');
        container.className = 'image-container';
        const img = document.createElement('img');
        img.src = block.src;
        img.alt = block.alt || '';
        container.appendChild(img);
        if (block.caption) {
            const cap = document.createElement('p');
            cap.className = 'image-caption';
            cap.textContent = block.caption;
            container.appendChild(cap);
        }
        return container;
    }

    function renderNoteBlock(block) {
        const div = document.createElement('div');
        div.className = 'note';
        div.innerHTML = block.html;
        return div;
    }

    function renderVideoCTA(block) {
        const a = document.createElement('a');
        a.href = block.url;
        a.className = 'video-cta';
        a.target = '_blank';
        a.innerHTML =
            '<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">' +
            '<path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>' +
            '</svg>' +
            '<div class="video-cta-content">' +
            '<span class="video-cta-title">' + esc(block.title) + '</span>' +
            '<span class="video-cta-desc">' + esc(block.description) + '</span>' +
            '</div>';
        return a;
    }

    function renderSampleLink(block) {
        const a = document.createElement('a');
        a.href = block.url;
        a.className = 'sample-link';
        a.target = '_blank';
        a.rel = 'noopener noreferrer';
        a.textContent = block.label;
        return a;
    }

    function renderConverterCTA(block) {
        const div = document.createElement('div');
        div.className = 'converter-cta';
        div.innerHTML =
            '<div class="converter-cta-icon">' +
            '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
            '<polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>' +
            '</svg>' +
            '</div>' +
            '<div class="converter-cta-text">' +
            '<h3>' + esc(block.title) + '</h3>' +
            '<p>' + esc(block.description) + '</p>' +
            '<a href="' + esc(block.url) + '" class="button-link">' + esc(block.button) + '</a>' +
            '</div>';
        return div;
    }

    function renderSection(section) {
        const frag = document.createDocumentFragment();

        section.blocks.forEach(function (block) {
            var el;
            switch (block.type) {
                case 'heading': {
                    el = document.createElement('h' + block.level);
                    if (section.id) el.id = section.id;
                    el.textContent = block.text;
                    break;
                }
                case 'paragraph': {
                    el = document.createElement('p');
                    el.innerHTML = block.html;
                    break;
                }
                case 'list': {
                    el = renderListBlock(block);
                    break;
                }
                case 'table': {
                    el = renderTableBlock(block);
                    break;
                }
                case 'image': {
                    el = renderImageBlock(block);
                    break;
                }
                case 'note': {
                    el = renderNoteBlock(block);
                    break;
                }
                case 'video-cta': {
                    el = renderVideoCTA(block);
                    break;
                }
                case 'sample-link': {
                    el = renderSampleLink(block);
                    break;
                }
                case 'converter-cta': {
                    el = renderConverterCTA(block);
                    break;
                }
                default:
                    return;
            }
            if (el) frag.appendChild(el);
        });

        return frag;
    }

    function renderTOC(data) {
        const div = document.createElement('div');
        div.className = 'toc';

        const h2 = document.createElement('h2');
        h2.textContent = 'Table of Contents';
        div.appendChild(h2);

        const ul = document.createElement('ul');
        data.nav.forEach(function (item) {
            const li = document.createElement('li');
            const a = document.createElement('a');
            a.href = '#' + item.id;
            a.textContent = item.label;
            li.appendChild(a);
            ul.appendChild(li);
        });
        div.appendChild(ul);
        return div;
    }

    function renderHeader(data) {
        const header = document.createElement('header');
        const div = document.createElement('div');
        const h1 = document.createElement('h1');
        h1.textContent = data.title;
        div.appendChild(h1);
        const p = document.createElement('p');
        p.textContent = data.description;
        div.appendChild(p);
        header.appendChild(div);

        const img = document.createElement('img');
        img.src = data.logo;
        img.alt = 'Odoo Logo';
        img.className = 'logo';
        header.appendChild(img);
        return header;
    }

    fetch('data.json')
        .then(function (r) { return r.json(); })
        .then(function (data) {
            app.appendChild(renderHeader(data));
            app.appendChild(renderTOC(data));
            data.sections.forEach(function (section) {
                var frag = renderSection(section);
                if (frag) app.appendChild(frag);
            });
        })
        .catch(function (err) {
            app.innerHTML = '<p style="color:red">Failed to load documentation data.</p>';
            console.error(err);
        });
})();
