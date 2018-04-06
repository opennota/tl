/* global pageNumber, nonce:true */

(function() {
  'use strict';

  const rowsPerPage = 50; // keep in sync with aligner.go

  const rowHTML = `<tr>
<td><i class="fa fa-times icon-remove"></i></td>
<td class="left"></td>
<td class="right"></td>
</tr>`;

  function flip(side) {
    return side === 'left' ? 'right' : 'left';
  }

  function appendWords($el, words) {
    $el.append(words.map(w => $('<span>').text(w)));
  }

  function columnUp($row, sel) {
    while ($row.length) {
      $row
        .prev()
        .find(sel)
        .html($row.find(sel).html());
      $row = $row.next();
    }
  }

  function moveRest($start, $to) {
    let el = $start.get(0);
    while (el) {
      const next = el.nextSibling;
      $to.append(el);
      el = next;
    }
  }

  function checkErr(xhr, status, err) {
    if (xhr.status == 409 /* Conflict */) {
      bootbox.alert({
        message: 'Achtung! The aligner is out of sync and will be reloaded.',
        backdrop: true,
        callback: () => {
          location.href = '/aligner';
        },
      });
    } else {
      bootbox.alert('Error: ' + err);
    }
  }

  function split(e) {
    if (e.ctrlKey || e.shiftKey || e.altKey) return;
    e.stopPropagation();
    const $span = $(e.target);
    const $td = $span.parent();
    const $tr = $td.parent();
    const side = $td.attr('class');
    $.ajax({
      method: 'POST',
      url: '/aligner',
      data: {
        op: 'split',
        side: side,
        page: pageNumber,
        row: $tr.index(),
        word: $span.index(),
        nonce: nonce++,
      },
    })
      .done(() => {
        const $newRow = $(rowHTML);
        const $nextRow = $tr.next();
        $tr.after($newRow);
        const $td = $newRow.find('.' + side);
        moveRest($span, $td);
        const sel = '.' + flip(side);
        columnUp($nextRow, sel);
        const $lastRow = $tr
          .parent()
          .children()
          .last();
        if ($lastRow.find('.' + side + ' span').length) {
          $lastRow.find(sel).html('');
        } else {
          $lastRow.remove();
        }
        const $rows = $tr.parent().children();
        if ($rows.length > rowsPerPage) {
          $rows.last().remove();
        }
      })
      .fail(checkErr);
  }

  function join(e) {
    const $td = $(e.currentTarget);
    const $tr = $td.parent();
    const side = $td.attr('class');
    $.ajax({
      method: 'POST',
      url: '/aligner',
      data: {
        op: 'join',
        side: side,
        page: pageNumber,
        row: $tr.index(),
        nonce: nonce++,
      },
    })
      .done(data => {
        const sel = '.' + side;
        const $td1 = $tr.find(sel);
        const $nextRow = $tr.next();
        if (data[0]) {
          appendWords($td1, data[0]);
        }
        columnUp($nextRow.next(), sel);

        const $lastRow = $tr
          .parent()
          .children()
          .last();
        if (data[1]) {
          const $td = $lastRow.find(sel);
          $td.html('');
          appendWords($td, data[1]);
        } else {
          if ($lastRow.find('.' + flip(side) + ' span').length) {
            $lastRow.find(sel).html('');
          } else {
            $lastRow.remove();
          }
        }
        getSelection().removeAllRanges();
      })
      .fail(checkErr);
  }

  function rm(e) {
    const $tr = $(e.target).closest('tr');
    $.ajax({
      method: 'POST',
      url: '/aligner',
      data: {
        op: 'rm',
        page: pageNumber,
        row: $tr.index(),
        nonce: nonce++,
      },
    })
      .done(data => {
        const $tbody = $tr.parent();
        $tr.remove();
        if (!data[0] && !data[1]) return;
        const $newRow = $(rowHTML);
        if (data[0]) {
          appendWords($newRow.find('.left'), data[0]);
        }
        if (data[1]) {
          appendWords($newRow.find('.right'), data[1]);
        }
        $tbody.append($newRow);
      })
      .fail(checkErr);
  }

  $(document).ready(() => {
    $('.aligner-table')
      .on('click', 'span', split)
      .on('click', 'td', e => {
        if (e.shiftKey && !(e.ctrlKey || e.altKey)) {
          join(e);
        }
      })
      .on('click', '.icon-remove', rm);
    $('.button-clear').on('click', () => {
      bootbox.confirm({
        message: 'Are you sure?',
        callback: result => {
          if (!result) return;
          $.ajax({
            method: 'POST',
            url: '/aligner',
            data: { op: 'clear', nonce: nonce++ },
          })
            .done(() => {
              location.href = '/aligner';
            })
            .fail(checkErr);
        },
      });
    });
  });
})();
// vim: ts=2 sts=2 sw=2 et