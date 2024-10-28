function searchTable() {
    var input, filter, table, tr, td, i, txtValue;
    input = document.getElementById("search-input");
    if (!input) return; //Added check for input element
    filter = input.value.toUpperCase();
    table = document.getElementById("syslog-table");
    if (!table) return; //Added check for table element
    tr = table.getElementsByTagName("tr");

    for (i = 1; i < tr.length; i++) {
        td = tr[i].getElementsByTagName("td");
        var display = false;
        for (var j = 0; j < td.length; j++) {
            if (td[j]) {
                txtValue = td[j].textContent || td[j].innerText;
                if (txtValue.toUpperCase().indexOf(filter) > -1) {
                    display = true;
                    break;
                }
            }
        }
        tr[i].style.display = display ? "" : "none";
    }
}

document.addEventListener('DOMContentLoaded', function() {
    document.body.addEventListener('htmx:afterSwap', function(event) {
        searchTable();
    });
});
