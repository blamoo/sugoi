function preloadImage(url) {
    img = new Image();
    img.src = url;
}

function isFullscreen() {
    if (document.fullscreenElement !== undefined) return !!document.fullscreenElement;
    if (document.webkitFullscreenElement !== undefined) return !!document.webkitFullscreenElement;
    if (document.mozFullScreenElement !== undefined) return !!document.mozFullScreenElement;
    if (document.msFullscreenElement !== undefined) return !!document.msFullscreenElement;
    return false;
}

function setThingRating(hash, rate) {
    return $.post('/thing/' + hash + '/rating.json', {
        rate: rate
    });
}

function thingAddMark(hash) {
    return $.post('/thing/' + hash + '/addMark.json');
}

function thingSubMark(hash) {
    return $.post('/thing/' + hash + '/subMark.json');
}

function updateMarksInput() {
    $('[data-marks-input]').each(function () {
        var $this = $(this);

        if ($this.attr('data-marks-input-loaded')) {
            return;
        }

        var hash = $this.data('marks-input');
        var $marks = $this.find('[data-marks]');
        var $sub = $this.find('[data-marks-sub]');
        var $add = $this.find('[data-marks-add]');

        $this.attr('data-marks-input-loaded', 1);

        $add.click(function (e) {
            e.preventDefault();
            $marks.html('<i class="fa-solid fa-spinner fa-spin loader"></i>');

            thingAddMark(hash)
                .done(function (data) {
                    $marks.html(data.Marks);
                });
        });

        $sub.click(function (e) {
            e.preventDefault();
            $marks.html('<i class="fa-solid fa-spinner fa-spin loader"></i>');

            thingSubMark(hash)
                .done(function (data) {
                    $marks.html(data.Marks);
                });
        });
    });
}

function setThingCover(hash, file) {
    return $.post('/thing/' + hash + '/cover.json', {
        file: file
    });
}

$.easing.slowOut = function (i) {
    return i * i;
}

$.fn.appendRatingForm = function (id, initialRating) {
    var $this = this;

    return new Promise(function (resolve, reject) {
        var $rateForm = $('<form class="text-nowrap" method="post">').attr("action", '/thing/' + id + '/rating.json?toggle=true');

        function updateButtons(stars) {
            var buttons = [];
            for (let i = 1; i <= 5; i++) {
                var $btn = $('<button class="rateButton" type="submit" name="rate">');
                $btn.attr('value', i);
                if (i <= stars) {
                    $btn.addClass('active');
                    $btn.html('<i class="fa-star fas"></i>');
                } else {
                    $btn.html('<i class="fa-star far"></i>');
                }
                buttons.push($btn);
            }

            $rateForm.html(buttons);
        }
        updateButtons(initialRating);

        $rateForm.submit(function (e) {
            e.preventDefault();
            if (!('submitter' in e.originalEvent) || !('value' in e.originalEvent.submitter) || !(e.originalEvent.submitter.value)) {
                return;
            }
            $rateForm.find('button').hide();
            $rateForm.append('<i class="fa-solid fa-spinner fa-spin loader"></i>');
            var iv = e.originalEvent.submitter.value

            var fd = new FormData(e.target);
            fd.set('rate', iv);

            $.ajax({
                url: $rateForm.attr('action'),
                type: 'POST',
                data: fd,
                processData: false,
                contentType: false
            })
                .done(function (data) {
                    iv = initialRating = data.Rating;
                    updateButtons(iv);
                    resolve(iv);
                })
                .fail(function () {
                    reject();
                })
                .always(function () {
                    $rateForm.find('button').show();
                    $rateForm.find('i.loader').remove();
                });
        });

        $rateForm.appendTo($this);
    });
}

const queryHistory = {
    list: [],
    initialized: false,
    initialize: function () {
        if (this.initialized) {
            return;
        }

        try {
            var str = localStorage.getItem("queryHistory.list");
            this.list = JSON.parse(str);
        } catch (error) {
            this.list = [];
        }

        if (!Array.isArray(this.list)) {
            this.list = [];
        }

        this.initialized = true;
    },
    first: function () {
        if (this.list.length === 0) {
            return null;
        } else {
            return this.list[0];
        }
    },
    push: function (url, label) {
        if (url === "/?" || url === "/") {
            return;
        }
        
        this.list = this.list.filter(function (val) {
            return val.label !== label;
        });

        this.list.unshift({url: url, label: label});
        this.save();
    },
    save: function () {
        this.list = this.list.slice(0, 6);
        localStorage.setItem("queryHistory.list", JSON.stringify(this.list));
    },
    removeByLabel: function (label) {
        this.list = this.list.filter(function (val) {
            return val.label !== label;
        });
        this.save();
    },
}

var $brandButton = $('#brandButton');
var $historyButton = $('#historyButton');
var $historyMenu = $('#historyMenu');

function updateQueryHistoryButton() {
    queryHistory.initialize();
    $historyMenu.empty();
    $historyMenu.append('<li><a class="dropdown-item" href="/">Home</a></li>');
    
    for (const item of queryHistory.list) {
        var $newItem = $('<a class="dropdown-item">').html(item.label).attr('href', item.url);
        $historyMenu.append($('<li class="d-flex align-items-center">').attr('data-label', item.label).html([
            $newItem,
            '<button type="button" class="btn-close fs-12px mx-2" data-action="remove"></button>'
        ]));
    }

    var first = queryHistory.first();
    if (first === null) {
        $brandButton.attr("href", '/');
    } else {
        if ('/' + location.search === first.url) {
            $brandButton.attr("href", '/');
        } else {
            $brandButton.attr("href", first.url);
        }
    }
}

$historyMenu.on('click', '[data-action="remove"]', function(e) {~
    e.preventDefault();
    e.stopPropagation();
    const $li = $(this).closest('[data-label]');

    queryHistory.removeByLabel($li.data('label'));
    $li.remove();
});

$(document).ready(function (e) {
    updateQueryHistoryButton();

    var $reindexStatus = $('#reindexStatus');
    var updateReindexStatus = function () {
        $.get('/system/reindexStatus').done(function (data) {
            if (data.Message) {
                $reindexStatus.show().html(data.Message);
            } else {
                $reindexStatus.hide().html('');
            }

            if (data.Stop) {
                $reindexStatus.hide().html('');
                clearInterval(statusInterval);
                return;
            }
        });
    }

    var statusInterval = setInterval(updateReindexStatus, 5000);
    updateReindexStatus();
    updateMarksInput();
});

function loadingAlert() {
    return '<div class="alert alert-info">Loading...</div>';
}
