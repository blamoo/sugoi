window.sugoi = window.sugoi || {};

var BLANK_IMAGE = 'data:image/gif;base64,R0lGODlhAQABAIEAAAAAAAAAAAAAAAAAACH5BAEAAAAALAAAAAABAAEAAAgEAAEEBAA7';

window.sugoi.reader = {
    params: null,
    pagesRead: new Set(),
    thingReadSent: false,
    prevKey: null,
    lastMove: 0,
    slideshowIsOn: false,
    slideshowInterval: null,
    slideshowPersistent: false,
    slideshowMs: 0,
    initialized: false,

    initialize: function () {
        if (this.initialized) {
            return;
        }

        this.$title = $('title');
        this.$target = $('#target');
        this.$fullscreenToaster = $('#fullscreenToaster');
        this.$fullscreenProgress = $('#fullscreenProgress');
        this.$fullscreenProgressBar = $('#fullscreenProgressBar').css({ width: "0%" });
        this.$fullscreenPageCounter = $('#fullscreenPageCounter');
        this.$loadingSpinner = $('#loadingSpinner');
        this.$targetImg = $('<img>').css({ maxWidth: '100%', maxHeight: '100%', objectFit: 'contain' }).hide().appendTo(this.$target);
        this.$navPrev = $('#navPrev');
        this.$navNext = $('#navNext');
        this.$navCurr = $('#navCurr');
        this.$navFirst = $('#navFirst');
        this.$navLast = $('#navLast');
        this.$stopSlideshowButton = $('#stopSlideshowButton');
        this.$navAll = $('#navPrev,#navNext,#navFirst,#navLast');
        this.$fullscreen = $('#fullscreen');
        this.$backButton = $('#backButton');
        this.$marksSubButton = $('#marksSubButton');
        this.$marksAddButton = $('#marksAddButton');
        this.$marksCounter = $('#marksCounter');
        this.$searchTagsContainer = $('#searchTagsContainer');

        this.defaultSlideshowSeconds = parseInt(this.$target.data('defaultSlideshowSeconds'), 10);

        this.$navAll.on('click', (e) => {
            e.preventDefault();
            this.setPage($(e.currentTarget).data('page'));
        });

        this.$targetImg.on('click', (e) => {
            e.preventDefault();
            var halfWidth = e.currentTarget.width / 2;

            if (e.originalEvent.offsetX > halfWidth) {
                this.setPage(this.params.page + 1);
            } else {
                this.setPage(this.params.page - 1);
            }
        });

        this.$fullscreen.on('click', (e) => {
            e.preventDefault();
            this.goFullScreen();
        });

        $(document.body).on('click', '[data-start-slideshow]', (e) => {
            this.startSlideshow(e.target.dataset.startSlideshow * 1000);
        });

        $(document.body).on('click', '[data-stop-slideshow]', (e) => {
            this.stopSlideshow();
        });

        window.addEventListener('popstate', (event) => {
            if ('page' in event.state) {
                this.setPage(event.state.page);
            }
        });

        window.addEventListener('keydown', (event) => this.onKeydown(event));

        this.$target[0].addEventListener('mousemove', (e) => {
            this.lastMove = Date.now();
            e.currentTarget.style.cursor = 'pointer';
        });

        setInterval(() => {
            if (this.lastMove + 3000 < Date.now()) { // 3 seconds
                this.$target[0].style.cursor = 'none';
            }
        }, 1000);

        this.initialized = true;
    },

    load: function (params) {
        this.initialize();

        if (this.slideshowInterval) {
            clearInterval(this.slideshowInterval);
            this.slideshowInterval = null;
        }

        this.params = params;
        this.pagesRead = new Set();
        this.thingReadSent = false;
        this.prevKey = null;
        this.slideshowIsOn = false;
        this.slideshowPersistent = false;
        this.slideshowMs = this.defaultSlideshowSeconds * 1000;

        this.$backButton.attr('href', params.backUrl);
        this.$marksSubButton.attr('data-marks-sub', params.hash);
        this.$marksAddButton.attr('data-marks-add', params.hash);
        this.$marksCounter.attr('data-marks-counter', params.hash).html(params.marks);
        this.$searchTagsContainer.html(params.searchTags.join(''));

        this.setPage(params.page);

        let ssp = sessionStorage.getItem('slideshowPersist');
        if (ssp !== null) {
            this.startSlideshow(parseInt(ssp), true);
        }
    },

    setPage: function (i) {
        var params = this.params;
        if (!(i in params.pages)) {
            this.stopSlideshow();
            return false;
        }

        params.page = i | 0;

        var expectedPrefix = params.prefix + "/" + params.page;
        if (location.pathname != expectedPrefix) {
            if (location.pathname === params.prefix) {
                history.replaceState({ page: params.page }, "", expectedPrefix + location.search);
            } else {
                history.pushState({ page: params.page }, "", expectedPrefix + location.search);
            }
        }

        this.$targetImg.show();
        this.$targetImg.attr('src', params.pages[params.page]);
        var pageText = "Page " + (params.page + 1) + "/" + params.pages.length;
        this.$fullscreenPageCounter.html((params.page + 1) + "/" + params.pages.length);
        this.$navCurr.html(pageText)
        this.$title.html("sugoi - " + params.title + " " + pageText);

        var prevPage = params.page - 1;
        if (prevPage < 0) {
            this.$navPrev.addClass('disabled').attr('href', 'javascript:;').removeData('page')
        } else {
            this.$navPrev.removeClass('disabled').attr('href', params.prefix + "/" + prevPage).data('page', prevPage)
        }

        var nextPage = params.page + 1;
        if (nextPage >= params.pages.length) {
            this.$navNext.addClass('disabled').attr('href', 'javascript:;').removeData('page')
        } else {
            this.$navNext.removeClass('disabled').attr('href', params.prefix + "/" + nextPage).data('page', nextPage)
        }

        var firstPage = 0;
        if (params.page <= firstPage) {
            this.$navFirst.addClass('disabled').attr('href', 'javascript:;').removeData('page')
        } else {
            this.$navFirst.removeClass('disabled').attr('href', params.prefix + "/" + firstPage).data('page', firstPage)
        }

        var lastPage = params.pages.length - 1;
        if (params.page >= lastPage) {
            this.$navLast.addClass('disabled').attr('href', 'javascript:;').removeData('page')
        } else {
            this.$navLast.removeClass('disabled').attr('href', params.prefix + "/" + lastPage).data('page', lastPage)
        }

        if (nextPage in params.pages) {
            preloadImage(params.pages[nextPage]);
        }

        if (prevPage in params.pages) {
            preloadImage(params.pages[prevPage]);
        }

        this.pagesRead.add(i);
        if (!this.thingReadSent && this.pagesRead.size >= params.readThreshold) {
            this.thingReadSent = true;

            $.ajax({
                url: '/thing/pushRead/' + params.hash,
                method: 'POST'
            });
        }

        this.resetSlideshow();
        return true;
    },

    onKeydown: function (event) {
        if (event.isComposing) {
            return;
        }

        var params = this.params;

        switch (event.key) {
            case "0":
            case "1":
            case "2":
            case "3":
            case "4":
            case "5":
                if (event.key == this.prevKey) {
                    setThingRating(params.hash, event.key)
                        .done((data) => this.toaster(data.Message))
                        .fail((data) => this.toaster(data.responseJSON.Error));
                } else {
                    this.toaster("Press " + event.key + " again to set the rating to " + event.key + " stars");
                }
                break;

            case "m":
                thingChangeMark(params.hash, 'add')
                    .done((data) => this.toaster(data.Message))
                    .fail((data) => this.toaster(data.responseJSON.Error));
                break;

            case "n":
                thingChangeMark(params.hash, 'sub')
                    .done((data) => this.toaster(data.Message))
                    .fail((data) => this.toaster(data.responseJSON.Error));
                break;

            case "p":
                this.toggleSlideshow(this.defaultSlideshowSeconds * 1000, true);
                event.preventDefault();
                break;

            case "c":
                if (event.key == this.prevKey) {
                    setThingCover(params.hash, params.pages[params.page])
                        .done((data) => this.toaster(data.Message))
                        .fail((data) => this.toaster(data.responseJSON.Error));
                } else {
                    this.toaster("Press " + event.key + " again to set the cover to " + params.pages[params.page]);
                }
                break;

            case "s":
                this.toggleSlideshow(this.defaultSlideshowSeconds * 1000);
                event.preventDefault();
                break;

            case "b":
                $(document.body).html("")
                window.location.replace("https://google.com/")
                break;

            case "r":
                window.location.href = params.backUrl;
                event.preventDefault();
                break;

            case "t":
                var first = window.sugoi.queryHistory.first();
                if (first === null) {
                    this.toaster("Search history is empty");
                } else {
                    window.location.href = first.url;
                }
                event.preventDefault();
                break;

            case "y":
                this.GoToRandom();
                event.preventDefault();
                break;

            case "f":
                this.goFullScreen();
                event.preventDefault();
                break;

            case "Home":
                this.setPage(0);
                event.preventDefault();
                break;

            case "End":
                this.setPage(params.pages.length - 1);
                event.preventDefault();
                break;

            case "ArrowLeft":
                this.setPage(params.page - 1);
                event.preventDefault();
                break;

            case "ArrowRight":
                this.setPage(params.page + 1);
                event.preventDefault();
                break;

            case " ":
            case "Escape":
                this.stopSlideshow();
                event.preventDefault();
                break;

            case "=":
            case "+":
                if (this.slideshowIsOn) {
                    this.slideshowMs += 1000;
                    this.slideshowMs = Math.max(1000, this.slideshowMs);
                    this.toaster("Slideshow time: " + this.slideshowMs / 1000 + "s", 1000);
                }
                this.resetSlideshow();
                event.preventDefault();
                break;

            case "-":
                if (this.slideshowIsOn) {
                    this.slideshowMs -= 1000;
                    this.slideshowMs = Math.max(1000, this.slideshowMs);
                    this.toaster("Slideshow time: " + this.slideshowMs / 1000 + "s", 1000);
                }
                this.resetSlideshow();
                event.preventDefault();
                break;
        }

        this.prevKey = event.key;
    },

    toggleSlideshow: function (delayInMs, persistent) {
        if (this.slideshowIsOn) {
            this.stopSlideshow();
        } else {
            this.startSlideshow(delayInMs, persistent);
        }
    },

    resetSlideshow: function () {
        var params = this.params;
        if (!this.slideshowIsOn) {
            return;
        }

        if (params.page + 1 == params.pages.length && !this.slideshowPersistent) {
            this.stopSlideshow();
            return;
        }

        if (this.slideshowInterval) {
            clearInterval(this.slideshowInterval);
            this.slideshowInterval = null;
        }
        this.$fullscreenProgressBar.stop().css({ width: "0%" }).animate({ width: "100%" }, this.slideshowMs, "linear");

        this.slideshowInterval = setTimeout(() => {
            if (!this.slideshowPersistent) {
                if (!this.setPage(params.page + 1)) {
                    this.stopSlideshow();
                }
            } else {
                if (params.page + 1 === params.pages.length) {
                    this.GoToRandom();
                } else {
                    this.setPage(params.page + 1);
                }
            }
        }, this.slideshowMs);

        this.$stopSlideshowButton.show();
        if (this.slideshowPersistent) {
            sessionStorage.setItem('slideshowPersist', this.slideshowMs);
        }
    },

    startSlideshow: function (delayInMs, persistent) {
        if (this.params.page + 1 == this.params.pages.length && !persistent) {
            return;
        }

        this.slideshowMs = delayInMs;
        this.slideshowIsOn = true;
        this.slideshowPersistent = !!persistent;
        this.resetSlideshow();
    },

    stopSlideshow: function () {
        if (this.slideshowInterval) {
            clearInterval(this.slideshowInterval);
            this.$fullscreenProgressBar.stop().css({ width: "0%" });
            this.slideshowInterval = null;
        }
        this.$stopSlideshowButton.hide();
        this.slideshowPersistent = false;
        sessionStorage.removeItem('slideshowPersist');
        this.slideshowIsOn = false;
    },

    goFullScreen: function () {
        this.$target[0].requestFullscreen();
    },

    GoToRandom: function () {
        var first = window.sugoi.queryHistory.first();
        var url = first ? first.url : '/';
        var separator = url.indexOf('?') === -1 ? '?' : '&';

        this.$target.css('min-height', this.$target.outerHeight());
        this.$targetImg.attr('src', BLANK_IMAGE);
        this.$searchTagsContainer.hide();
        this.$fullscreenPageCounter.addClass('d-none');
        this.$loadingSpinner.removeClass('d-none');

        $.get(url + separator + 'random=1&refresh=1')
            .done((data) => this.load(data))
            .fail(() => this.toaster("Failed to load a random thing"))
            .always(() => {
                this.$target.css('min-height', '');
                this.$searchTagsContainer.show();
                this.$fullscreenPageCounter.removeClass('d-none');
                this.$loadingSpinner.addClass('d-none');
            });
    },

    toaster: function (msg, timeout) {
        timeout = timeout || 5000;
        var $message = $('<div class="alert alert-primary" role="alert">').html(msg);
        $message.appendTo(this.$fullscreenToaster);
        $message.fadeOut(timeout, "slowOut", function () {
            $message.remove();
        });
    },
};
