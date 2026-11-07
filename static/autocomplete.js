window.sugoi = window.sugoi || {};

// Danbooru-style autocomplete for the search box: suggests values for the
// term under the cursor and replaces only that term when one is picked.
window.sugoi.autocomplete = {
    attach: function (input) {
        var $input = $(input);
        var $menu = $('<ul class="dropdown-menu sugoi-autocomplete"></ul>');
        var timer = null;
        var seq = 0;
        var items = [];
        var active = -1;
        var current = null; // the term being completed: {start, end, sign}

        $input.attr('autocomplete', 'off');
        $input.wrap('<div class="position-relative"></div>');
        $menu.insertAfter($input);

        function close() {
            $menu.removeClass('show').empty();
            items = [];
            active = -1;
        }

        function highlight(i) {
            active = i;
            $menu.find('.dropdown-item').removeClass('active');
            if (i >= 0) {
                $menu.find('.dropdown-item').eq(i).addClass('active');
            }
        }

        function render(list) {
            $menu.empty();
            items = list;
            active = -1;

            if (list.length === 0) {
                close();
                return;
            }

            list.forEach(function (item, i) {
                var $a = $('<a class="dropdown-item d-flex justify-content-between align-items-center gap-3" href="#"></a>');
                var $label = $('<span class="badge text-decoration-none"></span>')
                    .addClass('bg-tag-' + item.field)
                    .text(item.value);
                var $count = $('<small class="text-muted"></small>').text(item.count);
                $a.append($label, $count);

                // mousedown fires before the input's blur, keeping the menu alive
                $a.on('mousedown', function (event) {
                    event.preventDefault();
                    pick(i);
                });

                $('<li></li>').append($a).appendTo($menu);
            });

            $menu.addClass('show');
        }

        function pick(i) {
            var item = items[i];
            if (!item || !current) {
                return;
            }

            var value = $input.val();
            var replacement = current.sign + item.insert;
            var after = value.slice(current.end);
            if (!/^\s/.test(after)) {
                replacement += ' ';
            }

            $input.val(value.slice(0, current.start) + replacement + after);
            var caret = current.start + replacement.length;
            input.setSelectionRange(caret, caret);
            close();
        }

        function lookup() {
            current = window.sugoi.autocomplete.termAt($input.val(), input.selectionStart);
            if (!current || current.text.length === 0) {
                close();
                return;
            }

            var mySeq = ++seq;
            $.getJSON('/autocomplete.json', { q: current.text })
                .done(function (list) {
                    // ignore answers to keystrokes that were already superseded
                    if (mySeq === seq) {
                        render(list);
                    }
                })
                .fail(close);
        }

        $input.on('input', function () {
            clearTimeout(timer);
            timer = setTimeout(lookup, 150);
        });

        $input.on('keydown', function (event) {
            if (items.length === 0) {
                return;
            }

            switch (event.key) {
                case 'ArrowDown':
                    event.preventDefault();
                    highlight((active + 1) % items.length);
                    break;

                case 'ArrowUp':
                    event.preventDefault();
                    highlight(active <= 0 ? items.length - 1 : active - 1);
                    break;

                case 'Tab':
                    event.preventDefault();
                    pick(active >= 0 ? active : 0);
                    break;

                case 'Enter':
                    // with nothing highlighted, Enter submits the search as usual
                    if (active >= 0) {
                        event.preventDefault();
                        pick(active);
                    }
                    break;

                case 'Escape':
                    close();
                    break;
            }
        });

        $input.on('blur', close);
    },

    // termAt finds the search term around the caret. Terms are separated by
    // whitespace outside double quotes. Returns the term without its +/- sign,
    // where it starts and ends in the string, and the sign to keep.
    termAt: function (value, caret) {
        var start = 0;
        var inQuotes = false;

        for (var i = 0; i < caret; i++) {
            var c = value[i];
            if (c === '\\') {
                i++;
            } else if (c === '"') {
                inQuotes = !inQuotes;
            } else if (!inQuotes && /\s/.test(c)) {
                start = i + 1;
            }
        }

        var end = caret;
        while (end < value.length) {
            var d = value[end];
            if (d === '\\') {
                end += 2;
                continue;
            }
            if (d === '"') {
                inQuotes = !inQuotes;
            } else if (!inQuotes && /\s/.test(d)) {
                break;
            }
            end++;
        }
        end = Math.min(end, value.length);

        var text = value.slice(start, end);
        var sign = '';
        if (text[0] === '-' || text[0] === '+') {
            sign = text[0];
            text = text.slice(1);
        }

        return { start: start, end: end, sign: sign, text: text };
    },
};

$(function () {
    $('input[data-autocomplete]').each(function () {
        window.sugoi.autocomplete.attach(this);
    });
});
