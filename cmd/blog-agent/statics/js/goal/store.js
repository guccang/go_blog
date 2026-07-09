// statics/js/goal/store.js
const store = {
  state: {
    level: 'daily',
    period: '',
    view: 'detail',       // detail | list | review
    goal: null,
    goals: [],
    parentGoal: null,
    parentCandidates: [],
    review: null,
    loading: false,
  },

  _listeners: {},

  on(event, fn) {
    if (!this._listeners[event]) this._listeners[event] = [];
    this._listeners[event].push(fn);
    return () => this.off(event, fn);
  },

  off(event, fn) {
    const arr = this._listeners[event];
    if (arr) this._listeners[event] = arr.filter(f => f !== fn);
  },

  dispatch(event, data) {
    const arr = this._listeners[event];
    if (arr) arr.forEach(fn => fn(data));
  },

  setState(partial) {
    Object.assign(this.state, partial);
    this.dispatch('state:changed', this.state);
  },
};

export { store };
