import { isEmpty } from '@/utils/index'

export function timeFormat(fmt = 'yyyy-MM-dd HH:mm:ss', date = new Date()) {
    if (isEmpty(fmt)) fmt = 'yyyy-MM-dd HH:mm:ss'
    let o = {
        'M+': date.getMonth() + 1, //月份
        'd+': date.getDate(), //日
        'H+': date.getHours(), //小时
        'm+': date.getMinutes(), //分
        's+': date.getSeconds(), //秒
        'q+': Math.floor((date.getMonth() + 3) / 3), //季度
        'S': date.getMilliseconds() //毫秒
    }
    if (/(y+)/.test(fmt)) fmt = fmt.replace(RegExp.$1, (date.getFullYear() + '').substr(4 - RegExp.$1.length))
    for (let k in o)
        if (new RegExp('(' + k + ')').test(fmt)) fmt = fmt.replace(RegExp.$1, (RegExp.$1.length === 1) ? (o[k]) : (('00' + o[k]).substr(('' + o[k]).length)))
    return fmt
}

export function utc2local(utcTime) {
    if (utcTime instanceof Date || typeof utcTime === 'number') return new Date(utcTime)
    const value = String(utcTime).trim()
    const hasTimezone = /(?:Z|[+-]\d{2}:?\d{2})$/i.test(value)
    return new Date(hasTimezone ? value : value + 'Z')
}

export function getLocalTime(utcTime) {
    if (isEmpty(utcTime)) return null
    const localTime = utc2local(utcTime)
    if (Number.isNaN(localTime.getTime())) return null
    return timeFormat(null, localTime)
}
