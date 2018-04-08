(function() {
  'use strict';

  const months = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ];

  const zpad2 = n => {
    if (n >= 10) {
      return n;
    }
    return '0' + n;
  };

  const zpad4 = n => {
    if (n >= 1000) {
      return n;
    }
    if (n >= 100) {
      return '0' + n;
    }
    if (n >= 10) {
      return '00' + n;
    }
    return '000' + n;
  };

  const format = d => {
    let day = d.getDate();
    let month = months[d.getMonth()];
    let year = d.getFullYear();
    let h = d.getHours();
    let m = d.getMinutes();
    let s = d.getSeconds();
    return `${zpad2(day)}-${month}-${zpad4(year)} ${zpad2(h)}:${zpad2(
      m
    )}:${zpad2(s)}`;
  };

  const pretty = d => {
    let seconds = Math.floor((new Date() - d) / 1e3);
    let days = Math.floor(seconds / (60 * 60 * 24));
    if (days < 0) {
      return 'somewhen in the future';
    }
    if (days === 0) {
      if (seconds < 60 * 60) {
        let minutes = Math.floor(seconds / 60);
        if (minutes === 0) {
          return 'just now';
        }
        if (minutes === 1) {
          return '1 minute ago';
        }
        return minutes + ' minutes ago';
      }
      let hours = Math.floor(seconds / (60 * 60));
      if (hours === 1) {
        return '1 hour ago';
      }
      return hours + ' hours ago';
    }
    if (days < 7) {
      if (days === 1) {
        return 'Yesterday';
      }
      return days + ' days ago';
    }
    if (days < 31) {
      let weeks = Math.floor(days / 7);
      if (weeks === 1) {
        return '1 week ago';
      }
      return weeks + ' weeks ago';
    }
    return format(d);
  };

  $(document).ready(() => {
    setInterval(() => {
      $('time').each((_, el) => {
        const $el = $(el);
        const text = $el.text();
        const d = new Date($el.attr('datetime'));
        const newText = pretty(d);
        if (newText != text) {
          $el.text(newText);
        }
      });
    }, 60 * 1000);
  });
})();
// vim: ts=2 sts=2 sw=2 et
