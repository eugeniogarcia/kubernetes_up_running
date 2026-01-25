import React from 'react';

/**
 * Simple router implementation to replace react-router-component
 * Matches URL path to a handler component
 */

export class Location extends React.Component {
  render() {
    const { path, handler: Handler, ...props } = this.props;
    const currentPath = window.location.pathname;

    if (currentPath === path) {
      return <Handler {...props} />;
    }
    return null;
  }
}

export class Locations extends React.Component {
  constructor(props) {
    super(props);
    this.handlePopState = () => {
      if (this.props.onNavigation) {
        this.props.onNavigation();
      }
    };
  }

  componentDidMount() {
    window.addEventListener('popstate', this.handlePopState);
  }

  componentWillUnmount() {
    window.removeEventListener('popstate', this.handlePopState);
  }

  render() {
    const { children, onNavigation, ...props } = this.props;
    return (
      <div {...props}>
        {children}
      </div>
    );
  }
}

export class Link extends React.Component {
  handleClick = (e) => {
    e.preventDefault();
    const href = this.props.href;
    window.history.pushState({}, '', href);
    window.dispatchEvent(new PopStateEvent('popstate'));
  };

  render() {
    const { href, children, ...props } = this.props;
    return (
      <a href={href} onClick={this.handleClick} {...props}>
        {children}
      </a>
    );
  }
}
