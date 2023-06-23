let callAPI = function(method, callback){
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = callback;

    xhr.open('GET', '/api/' + method + '?format=json', true);
    xhr.setRequestHeader('Accept', 'application/json');
    
    if (window.env != 'prod'){
        xhr.setRequestHeader('X-Env', 'staging')
    }

    xhr.send();
}
