/**
 * Parse the time to string
 * @param {string} uri expamle: oss://path/to/data /path/to/data
 * @returns {string} path/to/data
 */

export function isArray(obj) {
  return obj instanceof Array
}

export function isListEqual(array) {
  // if the other array is a falsy value, return
  if (!array) { return false }

  // compare lengths - can save a lot of time
  if (this.length !== array.length) { return false }

  for (var i = 0, l = this.length; i < l; i++) {
    // Check if we have nested arrays
    if (this[i] instanceof Array && array[i] instanceof Array) {
    // recurse into the nested arrays
      if (!this[i].equals(array[i])) { return false }
    } else if (this[i] !== array[i]) {
    // Warning - two different object instances will never be equal: {x:20} != {x:20}
      return false
    }
  }
  return true
}

export function uniqObjectList(ol, byColumns) {
  function genKey(o, columns) {
    var k = ''
    columns.forEach(function(x) { k += ':' + o[x] })
    return k
  }
  var res = []
  var keySet = new Set()
  ol.forEach(function(o) {
    var key = genKey(o, byColumns)
    if (!keySet.has(key)) {
      keySet.add(key)
      res.push(o)
    }
  })
  return res
}

export function parseUri(uri) {
  var splitedUri = uri.split('://')
  if (splitedUri.length > 1) {
    return { type: splitedUri[0], path: splitedUri[1] }
  }
  return { type: 'other', path: uri }
}

export function isEmpty(value) {
  return (typeof (value) === 'undefined' || (typeof (value) === 'string' && !value) ||
    (value && value.constructor === Object && Object.keys(value).length === 0) ||
    (value && value.constructor === Array && value.length === 0) ||
   value === null || false)
}

export function addIfNotExist(targetList, valueToAdd) {
  if (targetList.indexOf(valueToAdd) === -1) {
    targetList.push(valueToAdd)
  }
}

/**
 * Parse the time to string
 * @param {(Object|string|number)} time
 * @param {string} cFormat
 * @returns {string | null}
 */
export function parseTime(time, cFormat) {
  if (arguments.length === 0 || !time) {
    return null
  }
  const format = cFormat || '{y}-{m}-{d} {h}:{i}:{s}'
  let date
  if (typeof time === 'object') {
    date = time
  } else {
    if ((typeof time === 'string')) {
      if ((/^[0-9]+$/.test(time))) {
        // support "1548221490638"
        time = parseInt(time)
      } else {
        // support safari
        // https://stackoverflow.com/questions/4310953/invalid-date-in-safari
        time = time.replace(new RegExp(/-/gm), '/')
      }
    }

    if ((typeof time === 'number') && (time.toString().length === 10)) {
      time = time * 1000
    }
    date = new Date(time)
  }
  const formatObj = {
    y: date.getFullYear(),
    m: date.getMonth() + 1,
    d: date.getDate(),
    h: date.getHours(),
    i: date.getMinutes(),
    s: date.getSeconds(),
    a: date.getDay()
  }
  const time_str = format.replace(/{([ymdhisa])+}/g, (result, key) => {
    const value = formatObj[key]
    // Note: getDay() returns 0 on Sunday
    if (key === 'a') { return ['日', '一', '二', '三', '四', '五', '六'][value ] }
    return value.toString().padStart(2, '0')
  })
  return time_str
}

/**
 * @param {number} time
 * @param {string} option
 * @returns {string}
 */
export function formatTime(time, option) {
  if (('' + time).length === 10) {
    time = parseInt(time) * 1000
  } else {
    time = +time
  }
  const d = new Date(time)
  const now = Date.now()

  const diff = (now - d) / 1000

  if (diff < 30) {
    return '刚刚'
  } else if (diff < 3600) {
    // less 1 hour
    return Math.ceil(diff / 60) + '分钟前'
  } else if (diff < 3600 * 24) {
    return Math.ceil(diff / 3600) + '小时前'
  } else if (diff < 3600 * 24 * 2) {
    return '1天前'
  }
  if (option) {
    return parseTime(time, option)
  } else {
    return (
      d.getMonth() +
      1 +
      '月' +
      d.getDate() +
      '日' +
      d.getHours() +
      '时' +
      d.getMinutes() +
      '分'
    )
  }
}

/**
 * @param {string} url
 * @returns {Object}
 */
export function param2Obj(url) {
  const search = decodeURIComponent(url.split('?')[1]).replace(/\+/g, ' ')
  if (!search) {
    return {}
  }
  const obj = {}
  const searchArr = search.split('&')
  searchArr.forEach(v => {
    const index = v.indexOf('=')
    if (index !== -1) {
      const name = v.substring(0, index)
      const val = v.substring(index + 1, v.length)
      obj[name] = val
    }
  })
  return obj
}
