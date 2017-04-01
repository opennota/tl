(function() {
  'use strict';
  function star(e) {
    let $icon = $(e.target);
    let book_id = $icon.data('book_id');
    let fid = $icon.data('fragment_id');
    $.ajax({
      method: 'POST',
      url: '/book/' + book_id + '/' + fid + '/star'
    }).done(() => {
      $icon.removeClass('x-star fa-star-o')
           .addClass('x-unstar fa-star');
    }).fail((xhr, status, err) => alert(err));
  }
  function unstar(e) {
    let $icon = $(e.target);
    let book_id = $icon.data('book_id');
    let fid = $icon.data('fragment_id');
    $.ajax({
      method: 'DELETE',
      url: '/book/' + book_id + '/' + fid + '/star'
    }).done(() => {
      $icon.removeClass('x-unstar fa-star')
           .addClass('x-star fa-star-o');
    }).fail((xhr, status, err) => alert(err));
  }
  $(document).ready(() => {
    $('.starred')
      .on('click', '.x-star', star)
      .on('click', '.x-unstar', unstar);
  });
})();

