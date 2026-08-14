window.sugoi = window.sugoi || {};

window.sugoi.queryHistory = {
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
        if (url === "/?" || url === "/" || !label || label.trim() === "") {
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
};
