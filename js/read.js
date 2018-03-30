/* global Popper, book_id */
(function() {
  'use strict';

  $(document).ready(() => {
    let popper;
    const $node = $('#popper');
    const $link = $node.find('.permalink');
    let prevTarget;
    $('.container').on('click', '.text', e => {
      const $target = $(e.target);
      e.stopPropagation();
      if (popper) {
        popper.destroy();
        popper = null;
        if (e.target === prevTarget) {
          $node.css('visibility', 'hidden');
          $target.removeClass('active');
          return;
        }
        if (prevTarget) {
          $(prevTarget).removeClass('active');
        }
      }
      prevTarget = e.target;
      $node.css('visibility', 'visible');
      $target.addClass('active');
      const fid = $target.data('fid');
      const seq = $target.data('seq');
      $link.attr('href', `/book/${book_id}/${fid}`);
      $link.text(`#${seq || fid}`);
      popper = new Popper(e.target, $node.get(0), {
        placement: 'left',
        modifiers: { offset: { offset: '0,5' } },
      });
    });
    $(document).on('click', () => {
      if (popper) {
        popper.destroy();
        popper = null;
        $node.css('visibility', 'hidden');
        $('.text.active').each((i, el) => $(el).removeClass('active'));
      }
    });
  });
})();
// vim: ts=2 sts=2 sw=2 et
