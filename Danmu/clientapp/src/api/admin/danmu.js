import request from '@/config/request'

const baseUrl = '/admin'

export function getAllVids() {
    return request.get(baseUrl + '/danmakulist/vids').then(({ data }) => data.data)
}

export function search(searchForm) {
    return request.get(baseUrl + '/danmakulist/baseselect', { params: searchForm }).then(({ data }) => data.data)
}

export function del(id) {
    return request.get(baseUrl + '/danmakuedit/delete', { params: { id } }).then(({ data }) => data.data)
}

export function getById(id) {
    return request.get(baseUrl + '/danmakuedit', { params: { id } }).then(({ data }) => data.data)
}

export function update(data) {
    return request.post(baseUrl + '/danmakuedit/edit', data).then(({ data }) => data.data)
}
